-- Skema Rilis 1. Tabel leave_* dan holidays ditambahkan di Rilis 2.

CREATE TABLE departments (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(100) NOT NULL,
  code VARCHAR(20) NOT NULL,
  head_id BIGINT UNSIGNED NULL,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_departments_code (code),
  KEY idx_departments_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE positions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(100) NOT NULL,
  department_id BIGINT UNSIGNED NULL,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_positions_department_id (department_id),
  KEY idx_positions_deleted_at (deleted_at),
  CONSTRAINT fk_positions_department FOREIGN KEY (department_id) REFERENCES departments (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE users (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  email VARCHAR(150) NOT NULL,
  password VARCHAR(100) NOT NULL,
  is_active TINYINT(1) NOT NULL DEFAULT 1,
  last_login_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_users_email (email),
  KEY idx_users_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE employees (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  nik VARCHAR(30) NOT NULL,
  name VARCHAR(150) NOT NULL,
  phone VARCHAR(30) NULL,
  department_id BIGINT UNSIGNED NULL,
  position_id BIGINT UNSIGNED NULL,
  manager_id BIGINT UNSIGNED NULL,
  join_date DATE NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  photo VARCHAR(255) NULL,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_employees_user (user_id),
  UNIQUE KEY uq_employees_nik (nik),
  KEY idx_employees_department_id (department_id),
  KEY idx_employees_position_id (position_id),
  KEY idx_employees_manager_id (manager_id),
  KEY idx_employees_deleted_at (deleted_at),
  CONSTRAINT fk_employees_user FOREIGN KEY (user_id) REFERENCES users (id),
  CONSTRAINT fk_employees_department FOREIGN KEY (department_id) REFERENCES departments (id),
  CONSTRAINT fk_employees_position FOREIGN KEY (position_id) REFERENCES positions (id),
  CONSTRAINT fk_employees_manager FOREIGN KEY (manager_id) REFERENCES employees (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE departments
  ADD CONSTRAINT fk_departments_head FOREIGN KEY (head_id) REFERENCES employees (id);

CREATE TABLE roles (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(30) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_roles_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_roles (
  user_id BIGINT UNSIGNED NOT NULL,
  role_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (user_id, role_id),
  CONSTRAINT fk_user_roles_user FOREIGN KEY (user_id) REFERENCES users (id),
  CONSTRAINT fk_user_roles_role FOREIGN KEY (role_id) REFERENCES roles (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE kpi_templates (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  department_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(150) NOT NULL,
  is_active TINYINT(1) NOT NULL DEFAULT 1,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_kpi_templates_department_id (department_id),
  KEY idx_kpi_templates_deleted_at (deleted_at),
  CONSTRAINT fk_kpi_templates_department FOREIGN KEY (department_id) REFERENCES departments (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE kpi_indicators (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  kpi_template_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(200) NOT NULL,
  unit VARCHAR(30) NULL,
  target DECIMAL(14,2) NOT NULL,
  weight DECIMAL(6,2) NOT NULL,
  direction VARCHAR(10) NOT NULL,
  source VARCHAR(15) NOT NULL DEFAULT 'manual',
  sort INT NOT NULL DEFAULT 0,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_kpi_indicators_template (kpi_template_id),
  KEY idx_kpi_indicators_deleted_at (deleted_at),
  CONSTRAINT fk_kpi_indicators_template FOREIGN KEY (kpi_template_id) REFERENCES kpi_templates (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE appraisal_periods (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(100) NOT NULL,
  type VARCHAR(15) NOT NULL DEFAULT 'monthly',
  start_date DATE NOT NULL,
  end_date DATE NOT NULL,
  status VARCHAR(15) NOT NULL DEFAULT 'draft',
  kpi_weight DECIMAL(5,2) NOT NULL,
  behavior_weight DECIMAL(5,2) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_appraisal_periods_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE kpi_results (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  period_id BIGINT UNSIGNED NOT NULL,
  employee_id BIGINT UNSIGNED NOT NULL,
  indicator_id BIGINT UNSIGNED NOT NULL,
  actual DECIMAL(14,2) NOT NULL,
  achievement DECIMAL(6,2) NOT NULL,
  note TEXT NULL,
  evidence_path VARCHAR(255) NULL,
  evidence_name VARCHAR(200) NULL,
  status VARCHAR(15) NOT NULL DEFAULT 'submitted',
  submitted_by BIGINT UNSIGNED NULL,
  validated_by BIGINT UNSIGNED NULL,
  validated_at DATETIME(3) NULL,
  validation_note VARCHAR(255) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_kpi_results_pei (period_id, employee_id, indicator_id),
  KEY idx_kpi_results_employee_id (employee_id),
  CONSTRAINT fk_kpi_results_period FOREIGN KEY (period_id) REFERENCES appraisal_periods (id),
  CONSTRAINT fk_kpi_results_employee FOREIGN KEY (employee_id) REFERENCES employees (id),
  CONSTRAINT fk_kpi_results_indicator FOREIGN KEY (indicator_id) REFERENCES kpi_indicators (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE behavior_aspects (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  name VARCHAR(100) NOT NULL,
  description VARCHAR(255) NULL,
  sort INT NOT NULL DEFAULT 0,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_behavior_aspects_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE appraisals (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  period_id BIGINT UNSIGNED NOT NULL,
  employee_id BIGINT UNSIGNED NOT NULL,
  appraiser_id BIGINT UNSIGNED NULL,
  kpi_score DECIMAL(6,2) NULL,
  behavior_score DECIMAL(6,2) NULL,
  final_score DECIMAL(6,2) NULL,
  grade VARCHAR(30) NULL,
  employee_comment TEXT NULL,
  manager_comment TEXT NULL,
  development_plan TEXT NULL,
  status VARCHAR(15) NOT NULL DEFAULT 'draft',
  self_submitted_at DATETIME(3) NULL,
  mgr_submitted_at DATETIME(3) NULL,
  finalized_at DATETIME(3) NULL,
  finalized_by BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_appraisals_pe (period_id, employee_id),
  KEY idx_appraisals_employee_id (employee_id),
  KEY idx_appraisals_status (status),
  CONSTRAINT fk_appraisals_period FOREIGN KEY (period_id) REFERENCES appraisal_periods (id),
  CONSTRAINT fk_appraisals_employee FOREIGN KEY (employee_id) REFERENCES employees (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE appraisal_scores (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  appraisal_id BIGINT UNSIGNED NOT NULL,
  aspect_id BIGINT UNSIGNED NOT NULL,
  self_score INT NULL,
  manager_score INT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_appraisal_scores_aa (appraisal_id, aspect_id),
  CONSTRAINT fk_appraisal_scores_appraisal FOREIGN KEY (appraisal_id) REFERENCES appraisals (id) ON DELETE CASCADE,
  CONSTRAINT fk_appraisal_scores_aspect FOREIGN KEY (aspect_id) REFERENCES behavior_aspects (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE notifications (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  type VARCHAR(50) NOT NULL,
  data JSON NULL,
  read_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_notifications_user_id (user_id),
  CONSTRAINT fk_notifications_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE activity_log (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  user_id BIGINT UNSIGNED NULL,
  action VARCHAR(60) NOT NULL,
  subject_type VARCHAR(60) NOT NULL,
  subject_id BIGINT UNSIGNED NOT NULL,
  old_values JSON NULL,
  new_values JSON NULL,
  PRIMARY KEY (id),
  KEY idx_activity_log_user_id (user_id),
  KEY idx_activity_log_action (action),
  KEY idx_activity_subject (subject_type, subject_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE password_resets (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  token_hash VARCHAR(64) NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  used_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_password_resets_token (token_hash),
  KEY idx_password_resets_user_id (user_id),
  CONSTRAINT fk_password_resets_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
