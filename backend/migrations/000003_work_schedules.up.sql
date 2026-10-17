-- Jadwal kerja per departemen (hari & jam kerja) dan izin sebagian hari berbasis jam.

CREATE TABLE work_schedules (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(100) NOT NULL,
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  break_start VARCHAR(5) NULL,
  break_end VARCHAR(5) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_work_schedules_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE work_schedule_days (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  schedule_id BIGINT UNSIGNED NOT NULL,
  weekday TINYINT NOT NULL,
  start_time VARCHAR(5) NOT NULL,
  end_time VARCHAR(5) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_work_schedule_days_sw (schedule_id, weekday),
  CONSTRAINT fk_work_schedule_days_schedule FOREIGN KEY (schedule_id) REFERENCES work_schedules (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE departments
  ADD COLUMN schedule_id BIGINT UNSIGNED NULL,
  ADD KEY idx_departments_schedule_id (schedule_id),
  ADD CONSTRAINT fk_departments_schedule FOREIGN KEY (schedule_id) REFERENCES work_schedules (id);

ALTER TABLE leave_requests
  ADD COLUMN start_time VARCHAR(5) NULL,
  ADD COLUMN end_time VARCHAR(5) NULL,
  MODIFY days DECIMAL(6,2) NOT NULL;

ALTER TABLE leave_balances
  MODIFY quota DECIMAL(6,2) NOT NULL DEFAULT 0,
  MODIFY used DECIMAL(6,2) NOT NULL DEFAULT 0;

ALTER TABLE leave_types
  MODIFY annual_quota DECIMAL(6,2) NULL,
  MODIFY attachment_min_days DECIMAL(6,2) NOT NULL DEFAULT 0,
  MODIFY max_days_per_request DECIMAL(6,2) NULL;

-- Dua jadwal bawaan; "Jam kantor" menjadi default untuk departemen yang belum memilih jadwal.
-- weekday: 0 = Minggu ... 6 = Sabtu; hari yang tidak terdaftar = libur mingguan.
-- Istirahat 12.00-13.00 tidak dihitung sebagai jam kerja (dipakai untuk izin per jam).
INSERT INTO work_schedules (created_at, updated_at, name, is_default, break_start, break_end) VALUES
  (NOW(3), NOW(3), 'Jam kantor (Sen-Jum 08.00-17.00)', 1, '12:00', '13:00'),
  (NOW(3), NOW(3), 'Jam kantor + Sabtu (Sen-Jum 09.00-17.00, Sab 09.00-15.00)', 0, '12:00', '13:00');

INSERT INTO work_schedule_days (created_at, updated_at, schedule_id, weekday, start_time, end_time)
SELECT NOW(3), NOW(3), s.id, d.weekday,
       CASE WHEN s.is_default = 1 THEN '08:00' ELSE '09:00' END,
       CASE WHEN s.is_default = 0 AND d.weekday = 6 THEN '15:00' ELSE '17:00' END
FROM work_schedules s
JOIN (SELECT 1 AS weekday UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4 UNION ALL SELECT 5 UNION ALL SELECT 6) d
  ON (d.weekday BETWEEN 1 AND 5) OR (d.weekday = 6 AND s.is_default = 0);
