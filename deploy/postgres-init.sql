# Edit the password, then run once as the postgres superuser:
#   sudo -u postgres psql -f deploy/postgres-init.sql

CREATE USER qc WITH PASSWORD 'replace-me';
CREATE DATABASE quality_control OWNER qc;
\c quality_control
ALTER SCHEMA public OWNER TO qc;
GRANT ALL ON SCHEMA public TO qc;
