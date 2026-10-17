SET FOREIGN_KEY_CHECKS = 0;
ALTER TABLE departments DROP FOREIGN KEY fk_departments_schedule, DROP KEY idx_departments_schedule_id, DROP COLUMN schedule_id;
ALTER TABLE leave_requests DROP COLUMN start_time, DROP COLUMN end_time, MODIFY days DECIMAL(5,1) NOT NULL;
ALTER TABLE leave_balances MODIFY quota DECIMAL(5,1) NOT NULL DEFAULT 0, MODIFY used DECIMAL(5,1) NOT NULL DEFAULT 0;
ALTER TABLE leave_types MODIFY annual_quota DECIMAL(5,1) NULL, MODIFY attachment_min_days DECIMAL(4,1) NOT NULL DEFAULT 0, MODIFY max_days_per_request DECIMAL(5,1) NULL;
DROP TABLE IF EXISTS work_schedule_days;
DROP TABLE IF EXISTS work_schedules;
SET FOREIGN_KEY_CHECKS = 1;
