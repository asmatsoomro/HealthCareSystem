-- Seed data for quick local testing
INSERT INTO patients (name) VALUES ('Alice'), ('Bob') ON CONFLICT DO NOTHING;
INSERT INTO physicians (name) VALUES ('Dr. Smith'), ('Dr. Jones') ON CONFLICT DO NOTHING;
INSERT INTO drugs (name) VALUES ('Amoxicillin'), ('Ibuprofen'), ('Metformin') ON CONFLICT DO NOTHING;

-- Link Dr. Smith to Alice and Bob; Dr. Jones to Bob only
INSERT INTO physician_patients (physician_id, patient_id)
SELECT p2.id, p1.id FROM physicians p2, patients p1 WHERE p2.name='Dr. Smith' AND p1.name IN ('Alice','Bob')
ON CONFLICT DO NOTHING;

INSERT INTO physician_patients (physician_id, patient_id)
SELECT p2.id, p1.id FROM physicians p2, patients p1 WHERE p2.name='Dr. Jones' AND p1.name IN ('Bob')
ON CONFLICT DO NOTHING;

-- Some prescriptions
WITH ids AS (
    SELECT
        (SELECT id FROM patients WHERE name='Alice') AS alice,
        (SELECT id FROM patients WHERE name='Bob') AS bob,
        (SELECT id FROM physicians WHERE name='Dr. Smith') AS dr_smith,
        (SELECT id FROM physicians WHERE name='Dr. Jones') AS dr_jones,
        (SELECT id FROM drugs WHERE name='Amoxicillin') AS amox,
        (SELECT id FROM drugs WHERE name='Ibuprofen') AS ibu,
        (SELECT id FROM drugs WHERE name='Metformin') AS met
)
INSERT INTO prescriptions (patient_id, physician_id, drug_id, quantity, sig, prescribed_at)
SELECT alice, dr_smith, amox, 20, '1 tab BID', NOW() - INTERVAL '3 days' FROM ids UNION ALL
SELECT alice, dr_smith, ibu, 30, 'PRN pain', NOW() - INTERVAL '2 days' FROM ids UNION ALL
SELECT bob, dr_jones, met, 60, '500mg BID', NOW() - INTERVAL '1 days' FROM ids ON CONFLICT DO NOTHING;
