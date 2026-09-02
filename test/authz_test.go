package test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stevenwilliam/marketing_calendar/internal/adapter/postgres"
	"github.com/stevenwilliam/marketing_calendar/internal/app"
)

// The permission matrix of 12-security.md §4, asserted for every role in both
// directions. The second half — what a role CANNOT reach — is the half that
// catches regressions, so it is written out explicitly rather than inferred.
var matrix = map[string]struct {
	can    []string
	cannot []string
}{
	"marketing_staff": {
		can:    []string{"promo.view", "promo.create", "target.view", "report.view", "report.export"},
		cannot: []string{"promo.manage", "target.manage", "import.run", "site.manage", "user.manage", "settings.manage", "audit.view", "force_release"},
	},
	"marketing_head": {
		can:    []string{"promo.view", "promo.create", "promo.manage", "target.manage", "report.export", "import.view"},
		cannot: []string{"import.run", "site.manage", "user.manage", "settings.manage", "audit.view", "force_release"},
	},
	"business_analyst": {
		can: []string{"promo.view", "target.view", "report.view", "report.export", "import.view"},
		// D30/Q30: the BA approves, but approval rights come from the CHAIN,
		// not from a permission. It must not gain promo.create by being in it.
		cannot: []string{"promo.create", "promo.manage", "target.manage", "import.run", "site.manage", "user.manage", "settings.manage", "audit.view", "force_release"},
	},
	"finance_head": {
		can:    []string{"promo.view", "target.view", "target.manage", "report.export", "import.view"},
		cannot: []string{"promo.create", "promo.manage", "import.run", "site.manage", "user.manage", "settings.manage", "audit.view", "force_release"},
	},
	"operation": {
		can:    []string{"promo.view", "target.view", "report.view", "report.export"},
		cannot: []string{"promo.create", "target.manage", "import.view", "import.run", "site.manage", "user.manage", "settings.manage", "audit.view", "force_release"},
	},
	"cfo": {
		// D35's compensating control: the CFO can see who bypassed a chain.
		can:    []string{"promo.view", "target.manage", "report.export", "audit.view"},
		cannot: []string{"promo.create", "promo.manage", "import.run", "site.manage", "user.manage", "settings.manage", "force_release"},
	},
	"it": {
		can: []string{"import.run", "import.view", "site.manage", "user.manage", "settings.manage", "audit.view", "promo.view", "target.view", "report.view"},
		// D38: IT administers the box; it has no business reason to carry the
		// sales history off it. And force_release stays with superadmin.
		cannot: []string{"report.export", "promo.create", "promo.manage", "target.manage", "force_release"},
	},
	"superadmin": {
		can:    []string{"force_release", "promo.create", "promo.manage", "target.manage", "report.export", "import.run", "site.manage", "user.manage", "settings.manage", "audit.view"},
		cannot: []string{},
	},
}

func TestPermissionMatrixBothDirections(t *testing.T) {
	g := db(t)
	ctx := context.Background()
	users := postgres.NewUserRepo(g)
	company := mustUUID(t, g, `SELECT company_id FROM company WHERE company_code = 'MAXX'`)

	for roleCode, want := range matrix {
		t.Run(roleCode, func(t *testing.T) {
			roleID := mustUUID(t, g, `SELECT role_id FROM role WHERE role_code = ?`, roleCode)
			uid := createApprover(t, g, roleID, company)

			p, err := users.Principal(ctx, uid)
			if err != nil {
				t.Fatal(err)
			}
			for _, perm := range want.can {
				if !p.Can(perm) {
					t.Errorf("%s must hold %s and does not", roleCode, perm)
				}
			}
			for _, perm := range want.cannot {
				if p.Can(perm) {
					t.Errorf("%s must NOT hold %s and does", roleCode, perm)
				}
			}
			if roleCode == "superadmin" && !p.IsSuperadmin {
				t.Error("the superadmin role must set IsSuperadmin")
			}
			if roleCode != "superadmin" && p.IsSuperadmin {
				t.Errorf("%s must not be treated as superadmin", roleCode)
			}
		})
	}
}

// BR-5.4 / D37: a user is assigned companies EXPLICITLY, and the resolved
// principal sees exactly those. There is no wildcard.
func TestCompanyScopeIsExplicitAndBounded(t *testing.T) {
	g := db(t)
	ctx := context.Background()
	users := postgres.NewUserRepo(g)

	maxx := mustUUID(t, g, `SELECT company_id FROM company WHERE company_code = 'MAXX'`)
	ruuma := mustUUID(t, g, `SELECT company_id FROM company WHERE company_code = 'RUUMA'`)
	role := mustUUID(t, g, `SELECT role_id FROM role WHERE role_code = 'operation'`)

	single := createApprover(t, g, role, maxx)
	p, err := users.Principal(ctx, single)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.CompanyIDs) != 1 || !p.InCompany(maxx) {
		t.Fatalf("a single-brand user sees %d companies", len(p.CompanyIDs))
	}
	if p.InCompany(ruuma) {
		t.Fatal("a Maxx-only user must not be in Ruuma's scope")
	}
	// BR-4.4a: the roles handed to the approval engine are the roles held IN
	// THAT COMPANY, never the union across companies.
	if len(p.RolesInCompany(ruuma)) != 0 {
		t.Fatal("RolesInCompany leaked a role into a company the user is not in")
	}
	if len(p.RolesInCompany(maxx)) != 1 {
		t.Fatal("RolesInCompany lost the role the user does hold")
	}

	// The database refuses the wildcard shape outright (D37).
	err = g.Exec(`INSERT INTO user_role (user_id, role_id, company_id) VALUES (?, ?, NULL)`,
		single, role).Error
	if err == nil {
		t.Fatal("a NULL company_id was accepted: the wildcard that D37 withdrew is back")
	}
}

// A user with no grants authenticates and sees nothing. Deny by default is the
// resting state, not an error condition (BR-5.1).
func TestUserWithNoGrantsSeesNothing(t *testing.T) {
	g := db(t)
	ctx := context.Background()
	users := postgres.NewUserRepo(g)
	master := postgres.NewMasterRepo(g)

	role := mustUUID(t, g, `SELECT role_id FROM role WHERE role_code = 'operation'`)
	company := mustUUID(t, g, `SELECT company_id FROM company WHERE company_code = 'MAXX'`)
	uid := createApprover(t, g, role, company)
	if err := g.Exec(`DELETE FROM user_role WHERE user_id = ?`, uid).Error; err != nil {
		t.Fatal(err)
	}
	p, err := users.Principal(ctx, uid)
	if err != nil {
		t.Fatalf("a user with no grants must still resolve, not error: %v", err)
	}
	if len(p.Permissions) != 0 || len(p.CompanyIDs) != 0 {
		t.Fatalf("an ungranted user holds %d permissions in %d companies",
			len(p.Permissions), len(p.CompanyIDs))
	}
	sites, err := master.Sites(ctx, p.CompanyIDs, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 {
		t.Fatalf("an ungranted user can see %d sites", len(sites))
	}
}

// IDOR: every list query is scoped by company IN THE QUERY, so another
// company's rows are not merely hidden by the handler — they never load.
func TestListsAreScopedInTheQuery(t *testing.T) {
	g := db(t)
	ctx := context.Background()
	master := postgres.NewMasterRepo(g)
	promos := postgres.NewPromoRepo(g)

	maxx := mustUUID(t, g, `SELECT company_id FROM company WHERE company_code = 'MAXX'`)
	ruuma := mustUUID(t, g, `SELECT company_id FROM company WHERE company_code = 'RUUMA'`)

	sites, err := master.Sites(ctx, []uuid.UUID{maxx}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) == 0 {
		t.Fatal("the fixture has no Maxx sites; this test would pass vacuously")
	}
	for _, s := range sites {
		if s.CompanyID != maxx {
			t.Fatalf("a %s site appeared in a Maxx-scoped query", s.CompanyID)
		}
	}

	groups, err := master.SiteGroups(ctx, []uuid.UUID{maxx}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, gr := range groups {
		if gr.CompanyID != maxx {
			t.Fatal("a group from another brand appeared in a Maxx-scoped query")
		}
	}

	plans, _, err := promos.List(ctx, app.PlanFilter{Companies: []uuid.UUID{ruuma}})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range plans {
		if p.CompanyID != ruuma {
			t.Fatal("a plan from another brand appeared in a Ruuma-scoped query")
		}
	}

	// An EMPTY company scope must return nothing, not everything. This is the
	// failure mode where a missing filter reads as "no filter".
	empty, err := master.Sites(ctx, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("an empty company scope returned %d sites: absent scope read as unrestricted", len(empty))
	}
}

// BR-5.5: a role may additionally be limited to a set of sites, and where one
// is set every read is filtered to those sites IN THE QUERY.
//
// The two empty cases are opposites and are easy to swap: no scope means all
// sites, while a scope that excludes everything requested must return nothing.
func TestSiteScopeFiltersInTheQuery(t *testing.T) {
	g := db(t)
	ctx := context.Background()
	master := postgres.NewMasterRepo(g)
	users := postgres.NewUserRepo(g)

	maxx := mustUUID(t, g, `SELECT company_id FROM company WHERE company_code = 'MAXX'`)
	role := mustUUID(t, g, `SELECT role_id FROM role WHERE role_code = 'operation'`)
	uid := createApprover(t, g, role, maxx)

	all, err := master.Sites(ctx, []uuid.UUID{maxx}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 2 {
		t.Fatalf("the fixture needs at least two Maxx sites, has %d", len(all))
	}

	// No scope set: the user sees every site in their company.
	p, err := users.Principal(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.SiteIDs) != 0 {
		t.Fatal("a new user must start unrestricted")
	}
	unrestricted, err := master.Sites(ctx, p.CompanyIDs, p.SiteIDs, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(unrestricted) != len(all) {
		t.Fatalf("an unrestricted user sees %d of %d sites: an empty scope was read as 'no sites'",
			len(unrestricted), len(all))
	}

	// Scope to ONE site.
	only := all[0]
	if err := g.Exec(`INSERT INTO user_site_scope (user_id, site_id) VALUES (?, ?)`,
		uid, only.SiteID).Error; err != nil {
		t.Fatal(err)
	}
	p, err = users.Principal(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.SiteIDs) != 1 {
		t.Fatalf("the scope did not reach the principal: %d sites", len(p.SiteIDs))
	}
	scoped, err := master.Sites(ctx, p.CompanyIDs, p.SiteIDs, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped) != 1 || scoped[0].SiteID != only.SiteID {
		t.Fatalf("a site-scoped user saw %d sites; the filter is not in the query", len(scoped))
	}

	// Narrowing within the scope is allowed; widening past it is not.
	if got := app.IntersectSites(*p, []uuid.UUID{only.SiteID}); len(got) != 1 || got[0] != only.SiteID {
		t.Fatal("a scoped user must be able to ask for a site inside their scope")
	}
	outside := all[1].SiteID
	got := app.IntersectSites(*p, []uuid.UUID{outside})
	if len(got) == 0 {
		t.Fatal("asking only for a forbidden site returned an EMPTY filter, which reads as 'no filter' downstream")
	}
	for _, s := range got {
		if s == outside {
			t.Fatal("a scoped user widened past their scope")
		}
	}
	// And that filter must genuinely return nothing.
	none, err := master.Sites(ctx, p.CompanyIDs, got, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("asking for a forbidden site returned %d sites", len(none))
	}
}
