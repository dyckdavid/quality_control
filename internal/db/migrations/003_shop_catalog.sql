-- Retire the generic starter defect list on databases that already applied 001.
-- Shop codes are inserted by 001 on a new database, and by the shop seed either way.

DELETE FROM defect_codes AS d
WHERE lower(d.code) IN (
    'vis-scr', 'vis-den', 'vis-crk', 'vis-dis', 'vis-con',
    'dim-oot', 'dim-oor', 'dim-mis',
    'fun-nop', 'fun-int',
    'mat-wrg', 'mat-mix',
    'pkg-dmg', 'pkg-lbl',
    'doc-mis', 'doc-inc'
)
AND NOT EXISTS (
    SELECT 1 FROM inspection_defects AS i WHERE i.defect_code_id = d.id
);

DELETE FROM products AS p
WHERE lower(p.part_number) = 'brk-100'
AND NOT EXISTS (SELECT 1 FROM lots AS l WHERE l.product_id = p.id)
AND NOT EXISTS (SELECT 1 FROM inspection_plans AS pl WHERE pl.product_id = p.id)
AND NOT EXISTS (SELECT 1 FROM nonconformances AS n WHERE n.product_id = p.id)
AND NOT EXISTS (SELECT 1 FROM checklists AS c WHERE c.product_id = p.id);
