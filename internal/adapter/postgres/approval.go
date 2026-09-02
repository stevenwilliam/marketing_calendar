package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
	"github.com/stevenwilliam/marketing_calendar/internal/domain/approval"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/apierror"
	"github.com/stevenwilliam/marketing_calendar/internal/platform/id"
	"gorm.io/gorm"
)

type ApprovalRepo struct{ db *gorm.DB }

func NewApprovalRepo(db *gorm.DB) *ApprovalRepo { return &ApprovalRepo{db: db} }

// ActiveChain returns the LATEST version of the active chain, for opening a
// new instance. An instance already open keeps its own version (BR-4.3).
func (r *ApprovalRepo) ActiveChain(ctx context.Context, companyID uuid.UUID, subject approval.SubjectType) (approval.Chain, uuid.UUID, error) {
	var versionID, chainID uuid.UUID
	row := r.db.WithContext(ctx).Raw(`
		SELECT cv.version_id, c.chain_id
		  FROM approval_chain c
		  JOIN approval_chain_version cv ON cv.chain_id = c.chain_id
		 WHERE c.company_id = ? AND c.subject_type = ? AND c.is_active
		 ORDER BY cv.version_no DESC LIMIT 1`, companyID, string(subject)).Row()
	if err := row.Scan(&versionID, &chainID); err != nil {
		if isNoRows(err) {
			return approval.Chain{}, uuid.Nil, apierror.New(apierror.CodeConflict,
				"tidak ada rantai persetujuan aktif untuk perusahaan ini")
		}
		return approval.Chain{}, uuid.Nil, err
	}
	c, err := r.ChainVersion(ctx, versionID)
	return c, chainID, err
}

func (r *ApprovalRepo) ChainVersion(ctx context.Context, versionID uuid.UUID) (approval.Chain, error) {
	c := approval.Chain{VersionID: versionID}
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT s.step_id, s.step_no, s.step_name, s.satisfaction,
		       COALESCE(array_agg(sr.role_id::text) FILTER (WHERE sr.role_id IS NOT NULL), '{}')
		  FROM approval_step s
		  LEFT JOIN approval_step_role sr ON sr.step_id = s.step_id
		 WHERE s.version_id = ?
		 GROUP BY s.step_id
		 ORDER BY s.step_no`, versionID).Rows()
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		var stepID uuid.UUID
		var s approval.Step
		var roleStrs []string
		if err := rows.Scan(&stepID, &s.StepNo, &s.Name, &s.Satisfaction, pqArray(&roleStrs)); err != nil {
			return c, err
		}
		for _, rs := range roleStrs {
			if u, err := uuid.Parse(rs); err == nil {
				s.RoleIDs = append(s.RoleIDs, u)
			}
		}
		c.Steps = append(c.Steps, s)
	}
	if len(c.Steps) == 0 {
		return c, apierror.New(apierror.CodeConflict, "rantai persetujuan tidak memiliki langkah")
	}
	return c, nil
}

func (r *ApprovalRepo) OpenInstance(ctx context.Context, in approval.Instance) (uuid.UUID, error) {
	iid := id.New()
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO approval_instance (instance_id, version_id, company_id, subject_type,
		    subject_id, created_by, current_step_no, status)
		VALUES (?, ?, ?, ?, ?, ?, 1, 'PENDING')`,
		iid, in.VersionID, in.CompanyID, string(in.SubjectType), in.SubjectID, in.CreatedBy).Error
	return iid, err
}

func (r *ApprovalRepo) InstanceByID(ctx context.Context, iid uuid.UUID) (*approval.Instance, error) {
	return r.loadInstance(ctx, r.db.WithContext(ctx), `WHERE instance_id = ?`, iid)
}

func (r *ApprovalRepo) InstanceBySubject(ctx context.Context, t approval.SubjectType, sid uuid.UUID) (*approval.Instance, error) {
	return r.loadInstance(ctx, r.db.WithContext(ctx),
		`WHERE subject_type = ? AND subject_id = ?`, string(t), sid)
}

func (r *ApprovalRepo) loadInstance(ctx context.Context, tx *gorm.DB, where string, args ...any) (*approval.Instance, error) {
	var in approval.Instance
	row := tx.Raw(`
		SELECT instance_id, version_id, company_id, subject_type, subject_id,
		       created_by, current_step_no, status
		  FROM approval_instance `+where, args...).Row()
	if err := row.Scan(&in.InstanceID, &in.VersionID, &in.CompanyID, &in.SubjectType,
		&in.SubjectID, &in.CreatedBy, &in.CurrentStepNo, &in.Status); err != nil {
		if isNoRows(err) {
			return nil, apierror.NotFound("instansi persetujuan")
		}
		return nil, err
	}
	events, err := tx.Raw(`
		SELECT step_no, action, actor_id, COALESCE(reason, ''), occurred_at
		  FROM approval_event WHERE instance_id = ? ORDER BY occurred_at`, in.InstanceID).Rows()
	if err != nil {
		return nil, err
	}
	defer events.Close()
	for events.Next() {
		var e approval.Event
		var actor uuid.NullUUID
		if err := events.Scan(&e.StepNo, &e.Action, &actor, &e.Reason, &e.OccurredAt); err != nil {
			return nil, err
		}
		if actor.Valid {
			a := actor.UUID
			e.ActorID = &a
		}
		e.InstanceID = in.InstanceID
		in.Events = append(in.Events, e)
	}
	return &in, nil
}

// Decide is BR-4.10's concurrency control.
//
// SELECT ... FOR UPDATE on the instance serialises two approvers acting at the
// same moment: the second waits, then re-reads state that already includes the
// first decision, so the domain refuses it. The unique index on
// (instance, step, actor) is the second line — if the lock is ever removed,
// the database still refuses a double approval rather than silently accepting
// it. Belt and braces, deliberately.
func (r *ApprovalRepo) Decide(ctx context.Context, instanceID uuid.UUID, ip string,
	fn func(*approval.Instance, approval.Chain) (approval.Decision, error)) (approval.Decision, error) {

	var out approval.Decision
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// The lock. Everything after this reads committed state.
		var locked uuid.UUID
		if err := tx.Raw(`SELECT instance_id FROM approval_instance WHERE instance_id = ? FOR UPDATE`,
			instanceID).Row().Scan(&locked); err != nil {
			if isNoRows(err) {
				return apierror.NotFound("instansi persetujuan")
			}
			return err
		}

		in, err := r.loadInstance(ctx, tx, `WHERE instance_id = ?`, instanceID)
		if err != nil {
			return err
		}
		chain, err := r.chainVersionTx(tx, in.VersionID)
		if err != nil {
			return err
		}

		d, err := fn(in, chain)
		if err != nil {
			return err
		}

		var ipVal any
		if ip != "" {
			ipVal = ip
		}
		for _, e := range d.Events {
			var reason any
			if e.Reason != "" {
				reason = e.Reason
			}
			err := tx.Exec(`
				INSERT INTO approval_event (event_id, instance_id, step_no, action,
				    actor_id, reason, ip_address, occurred_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				id.New(), e.InstanceID, e.StepNo, string(e.Action),
				nullUUID(e.ActorID), reason, ipVal, e.OccurredAt).Error
			if err != nil {
				if isUniqueViolation(err) {
					// The partial unique index fired: this approver already
					// decided this step. Report it as the domain error, never
					// as a driver message.
					return apierror.New(apierror.CodeAlreadyDecided,
						approval.ErrAlreadyDecided.Error())
				}
				return err
			}
		}

		var closedAt any
		if d.NewStatus != approval.StatusPending {
			closedAt = d.Events[len(d.Events)-1].OccurredAt
		}
		if err := tx.Exec(`
			UPDATE approval_instance
			   SET current_step_no = ?, status = ?, closed_at = ?
			 WHERE instance_id = ?`,
			d.NextStepNo, string(d.NewStatus), closedAt, instanceID).Error; err != nil {
			return err
		}
		out = d
		return nil
	})
	return out, err
}

func (r *ApprovalRepo) chainVersionTx(tx *gorm.DB, versionID uuid.UUID) (approval.Chain, error) {
	c := approval.Chain{VersionID: versionID}
	rows, err := tx.Raw(`
		SELECT s.step_no, s.step_name, s.satisfaction,
		       COALESCE(array_agg(sr.role_id::text) FILTER (WHERE sr.role_id IS NOT NULL), '{}')
		  FROM approval_step s
		  LEFT JOIN approval_step_role sr ON sr.step_id = s.step_id
		 WHERE s.version_id = ?
		 GROUP BY s.step_id ORDER BY s.step_no`, versionID).Rows()
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		var s approval.Step
		var roleStrs []string
		if err := rows.Scan(&s.StepNo, &s.Name, &s.Satisfaction, pqArray(&roleStrs)); err != nil {
			return c, err
		}
		for _, rs := range roleStrs {
			if u, err := uuid.Parse(rs); err == nil {
				s.RoleIDs = append(s.RoleIDs, u)
			}
		}
		c.Steps = append(c.Steps, s)
	}
	return c, nil
}

func (r *ApprovalRepo) Events(ctx context.Context, instanceID uuid.UUID) ([]app.ApprovalEventRow, error) {
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT e.step_no, COALESCE(s.step_name, ''), e.action,
		       COALESCE(u.full_name, 'Sistem'), COALESCE(e.reason, ''), e.occurred_at
		  FROM approval_event e
		  JOIN approval_instance i ON i.instance_id = e.instance_id
		  LEFT JOIN approval_step s ON s.version_id = i.version_id AND s.step_no = e.step_no
		  LEFT JOIN app_user u ON u.user_id = e.actor_id
		 WHERE e.instance_id = ? ORDER BY e.occurred_at`, instanceID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.ApprovalEventRow
	for rows.Next() {
		var e app.ApprovalEventRow
		if err := rows.Scan(&e.StepNo, &e.StepName, &e.Action, &e.ActorName, &e.Reason, &e.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// Inbox lists what is waiting for THIS caller: instances whose current step
// names a role the caller holds IN THAT INSTANCE'S COMPANY (BR-4.4a), minus
// the plans they created themselves (BR-4.11) and the steps they have already
// decided (BR-4.10). Filtering in the query, not afterwards.
func (r *ApprovalRepo) Inbox(ctx context.Context, p app.Principal, limit, offset int) ([]app.PlanRow, int, error) {
	if len(p.Grants) == 0 {
		return nil, 0, nil
	}
	// The pair (role, company) is the unit of eligibility (BR-4.4a): holding
	// `operation` for Maxx Coffee must not match a Ruuma instance, and
	// matching role and company independently would do exactly that.
	pairList := make([]string, 0, len(p.Grants))
	for _, g := range p.Grants {
		pairList = append(pairList, g.RoleID.String()+":"+g.CompanyID.String())
	}
	pairs := textList(pairList)

	where := `
		 WHERE p.status = 'PENDING'
		   AND ai.status = 'PENDING'
		   AND p.created_by <> ?
		   AND EXISTS (
		       SELECT 1 FROM approval_step s
		         JOIN approval_step_role sr ON sr.step_id = s.step_id
		        WHERE s.version_id = ai.version_id
		          AND s.step_no = ai.current_step_no
		          AND (sr.role_id::text || ':' || ai.company_id::text) = ANY(?::text[])
		   )
		   AND NOT EXISTS (
		       SELECT 1 FROM approval_event e
		        WHERE e.instance_id = ai.instance_id
		          AND e.step_no = ai.current_step_no
		          AND e.action = 'APPROVE' AND e.actor_id = ?
		   )`

	var total int64
	err := r.db.WithContext(ctx).Raw(`
		SELECT count(*) FROM promotion_plan p
		  JOIN promotion_plan_version v ON v.version_id = p.current_version_id
		  JOIN approval_instance ai ON ai.instance_id = p.approval_instance_id`+where,
		p.UserID, pairs, p.UserID).Scan(&total).Error
	if err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.WithContext(ctx).Raw(
		planSelect+where+` ORDER BY v.start_date LIMIT ? OFFSET ?`,
		p.UserID, pairs, p.UserID, limit, offset).Rows()
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []app.PlanRow
	for rows.Next() {
		row, err := scanPlan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, int(total), nil
}

func (r *ApprovalRepo) ChainConfig(ctx context.Context, companyID uuid.UUID, subject approval.SubjectType) ([]app.ChainStepRow, int, error) {
	var versionID uuid.UUID
	var versionNo int
	row := r.db.WithContext(ctx).Raw(`
		SELECT cv.version_id, cv.version_no
		  FROM approval_chain c JOIN approval_chain_version cv ON cv.chain_id = c.chain_id
		 WHERE c.company_id = ? AND c.subject_type = ? AND c.is_active
		 ORDER BY cv.version_no DESC LIMIT 1`, companyID, string(subject)).Row()
	if err := row.Scan(&versionID, &versionNo); err != nil {
		if isNoRows(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT s.step_no, s.step_name, s.satisfaction,
		       COALESCE(array_agg(sr.role_id::text) FILTER (WHERE sr.role_id IS NOT NULL), '{}'),
		       COALESCE(array_agg(r.label_id) FILTER (WHERE r.role_id IS NOT NULL), '{}')
		  FROM approval_step s
		  LEFT JOIN approval_step_role sr ON sr.step_id = s.step_id
		  LEFT JOIN role r ON r.role_id = sr.role_id
		 WHERE s.version_id = ?
		 GROUP BY s.step_id ORDER BY s.step_no`, versionID).Rows()
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []app.ChainStepRow
	for rows.Next() {
		var s app.ChainStepRow
		var roleStrs []string
		if err := rows.Scan(&s.StepNo, &s.StepName, &s.Satisfaction, pqArray(&roleStrs), pqArray(&s.RoleLabels)); err != nil {
			return nil, 0, err
		}
		for _, rs := range roleStrs {
			if u, err := uuid.Parse(rs); err == nil {
				s.RoleIDs = append(s.RoleIDs, u)
			}
		}
		out = append(out, s)
	}
	return out, versionNo, nil
}

// SaveChain writes a NEW VERSION rather than editing the current one. That is
// BR-4.3: instances already open keep pointing at the version they started
// with, so an administrator cannot remove an inconvenient approver from a plan
// already in flight.
func (r *ApprovalRepo) SaveChain(ctx context.Context, companyID uuid.UUID, subject approval.SubjectType, steps []app.ChainStepRow, actor uuid.UUID) (int, error) {
	if len(steps) == 0 {
		return 0, apierror.Validation("rantai persetujuan harus memiliki minimal satu langkah", nil)
	}
	var newVersionNo int
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var chainID uuid.UUID
		row := tx.Raw(`SELECT chain_id FROM approval_chain
		                WHERE company_id = ? AND subject_type = ? AND is_active FOR UPDATE`,
			companyID, string(subject)).Row()
		if err := row.Scan(&chainID); err != nil {
			if !isNoRows(err) {
				return err
			}
			chainID = id.New()
			if err := tx.Exec(`
				INSERT INTO approval_chain (chain_id, company_id, subject_type, chain_name, is_active)
				VALUES (?, ?, ?, ?, true)`,
				chainID, companyID, string(subject), "Rantai "+string(subject)).Error; err != nil {
				return err
			}
		}
		if err := tx.Raw(`SELECT COALESCE(max(version_no), 0) + 1
		                    FROM approval_chain_version WHERE chain_id = ?`, chainID).
			Scan(&newVersionNo).Error; err != nil {
			return err
		}
		versionID := id.New()
		if err := tx.Exec(`
			INSERT INTO approval_chain_version (version_id, chain_id, version_no, created_by)
			VALUES (?, ?, ?, ?)`, versionID, chainID, newVersionNo, actor).Error; err != nil {
			return err
		}
		for i, s := range steps {
			stepID := id.New()
			sat := s.Satisfaction
			if sat != approval.AllOf {
				sat = approval.AnyOf
			}
			if err := tx.Exec(`
				INSERT INTO approval_step (step_id, version_id, step_no, step_name, satisfaction)
				VALUES (?, ?, ?, ?, ?)`,
				stepID, versionID, i+1, s.StepName, string(sat)).Error; err != nil {
				return err
			}
			if len(s.RoleIDs) == 0 {
				return apierror.Validation("setiap langkah harus memiliki minimal satu peran", nil)
			}
			for _, rid := range s.RoleIDs {
				if err := tx.Exec(`
					INSERT INTO approval_step_role (step_id, role_id) VALUES (?, ?)
					ON CONFLICT DO NOTHING`, stepID, rid).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	return newVersionNo, err
}
