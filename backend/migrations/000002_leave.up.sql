-- Rilis 2: izin & cuti, hari libur, dan pengaturan aplikasi (saklar notifikasi).

CREATE TABLE leave_types (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(100) NOT NULL,
  code VARCHAR(20) NOT NULL,
  annual_quota DECIMAL(5,1) NULL,
  deducts_balance TINYINT(1) NOT NULL DEFAULT 0,
  requires_attachment TINYINT(1) NOT NULL DEFAULT 0,
  attachment_min_days DECIMAL(4,1) NOT NULL DEFAULT 0,
  allow_half_day TINYINT(1) NOT NULL DEFAULT 0,
  min_tenure_months INT NOT NULL DEFAULT 0,
  max_days_per_request DECIMAL(5,1) NULL,
  sort INT NOT NULL DEFAULT 0,
  is_active TINYINT(1) NOT NULL DEFAULT 1,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_leave_types_code (code),
  KEY idx_leave_types_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE leave_balances (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  employee_id BIGINT UNSIGNED NOT NULL,
  leave_type_id BIGINT UNSIGNED NOT NULL,
  year INT NOT NULL,
  quota DECIMAL(5,1) NOT NULL DEFAULT 0,
  used DECIMAL(5,1) NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  UNIQUE KEY uq_leave_balances_etn (employee_id, leave_type_id, year),
  CONSTRAINT fk_leave_balances_employee FOREIGN KEY (employee_id) REFERENCES employees (id),
  CONSTRAINT fk_leave_balances_type FOREIGN KEY (leave_type_id) REFERENCES leave_types (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE leave_requests (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  number VARCHAR(30) NULL,
  employee_id BIGINT UNSIGNED NOT NULL,
  leave_type_id BIGINT UNSIGNED NOT NULL,
  start_date DATE NOT NULL,
  end_date DATE NOT NULL,
  half_day TINYINT(1) NOT NULL DEFAULT 0,
  half_session VARCHAR(10) NULL,
  days DECIMAL(5,1) NOT NULL,
  reason TEXT NULL,
  attachment_path VARCHAR(255) NULL,
  attachment_name VARCHAR(200) NULL,
  substitute_id BIGINT UNSIGNED NULL,
  status VARCHAR(20) NOT NULL,
  deducted TINYINT(1) NOT NULL DEFAULT 0,
  cancelled_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_leave_requests_number (number),
  KEY idx_leave_requests_emp_start (employee_id, start_date),
  KEY idx_leave_requests_status (status),
  KEY idx_leave_requests_range (start_date, end_date),
  CONSTRAINT fk_leave_requests_employee FOREIGN KEY (employee_id) REFERENCES employees (id),
  CONSTRAINT fk_leave_requests_type FOREIGN KEY (leave_type_id) REFERENCES leave_types (id),
  CONSTRAINT fk_leave_requests_substitute FOREIGN KEY (substitute_id) REFERENCES employees (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE leave_approvals (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  leave_request_id BIGINT UNSIGNED NOT NULL,
  approver_id BIGINT UNSIGNED NOT NULL,
  level TINYINT NOT NULL,
  action VARCHAR(15) NOT NULL,
  note VARCHAR(500) NULL,
  acted_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_leave_approvals_request (leave_request_id),
  KEY idx_leave_approvals_approver (approver_id),
  CONSTRAINT fk_leave_approvals_request FOREIGN KEY (leave_request_id) REFERENCES leave_requests (id) ON DELETE CASCADE,
  CONSTRAINT fk_leave_approvals_approver FOREIGN KEY (approver_id) REFERENCES employees (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE holidays (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  date DATE NOT NULL,
  name VARCHAR(150) NOT NULL,
  type VARCHAR(15) NOT NULL DEFAULT 'nasional',
  PRIMARY KEY (id),
  UNIQUE KEY uq_holidays_date (date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE app_settings (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(60) NOT NULL,
  value VARCHAR(255) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_app_settings_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
