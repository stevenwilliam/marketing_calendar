-- D59. The daily-achievement bands drop from three to two, split at a single
-- green line, and that line becomes a parameter rather than a constant.
--
-- Steven retuned this threshold twice inside one session — 70/100, then a
-- single 80 — which is exactly the signal CLAUDE.md §7 describes: a value that
-- changes without a code change belongs in sys_parameters. The next retune is
-- a row edit in Pengaturan, not a deploy.
INSERT INTO sys_parameters (param_key, param_value, value_type, description, is_secret)
VALUES ('promo.achievement_green_bps', '8000', 'int',
        'Batas capaian harian yang dihitung tercapai, dalam basis poin. 8000 = 80%. Di bawahnya merah, pada atau di atasnya hijau (BR-7.5a)',
        FALSE)
ON CONFLICT (param_key) DO NOTHING;
