#!/bin/bash

# Migration runner script for Twitter Clone
# Usage: ./run-migrations.sh [environment]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
ENV=${1:-development}
MIGRATIONS_DIR="./migrations"
MIGRATION_TABLE="schema_migrations"

# Load environment-specific config
case $ENV in
    development)
        DB_HOST="localhost"
        DB_PORT="5432"
        DB_USER="postgres"
        DB_PASS="postgres"
        ;;
    staging)
        DB_HOST=${STAGING_DB_HOST:-"postgres.staging.svc.cluster.local"}
        DB_PORT=${STAGING_DB_PORT:-"5432"}
        DB_USER=${STAGING_DB_USER}
        DB_PASS=${STAGING_DB_PASS}
        ;;
    production)
        DB_HOST=${PROD_DB_HOST}
        DB_PORT=${PROD_DB_PORT:-"5432"}
        DB_USER=${PROD_DB_USER}
        DB_PASS=${PROD_DB_PASS}
        ;;
    *)
        echo -e "${RED}Unknown environment: $ENV${NC}"
        exit 1
        ;;
esac

# Databases to migrate
DATABASES=("users_db" "tweets_db" "notifications_db" "media_db")

echo -e "${YELLOW}Running migrations for environment: $ENV${NC}"
echo "Database host: $DB_HOST:$DB_PORT"
echo ""

# Function to run migration
run_migration() {
    local db=$1
    local migration_file=$2
    local migration_name=$(basename $migration_file .sql)
    
    echo -e "  Applying: $migration_name"
    
    # Check if migration already applied
    already_applied=$(PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $db -t -c \
        "SELECT COUNT(*) FROM $MIGRATION_TABLE WHERE name = '$migration_name';" 2>/dev/null || echo "0")
    
    if [ "$already_applied" -gt 0 ]; then
        echo -e "  ${YELLOW}Already applied, skipping${NC}"
        return
    fi
    
    # Apply migration
    PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $db -f $migration_file
    
    # Record migration
    PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $db -c \
        "INSERT INTO $MIGRATION_TABLE (name, applied_at) VALUES ('$migration_name', NOW());"
    
    echo -e "  ${GREEN}✓ Applied successfully${NC}"
}

# Function to create migration table
create_migration_table() {
    local db=$1
    
    PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $db -c \
        "CREATE TABLE IF NOT EXISTS $MIGRATION_TABLE (
            id SERIAL PRIMARY KEY,
            name VARCHAR(255) NOT NULL UNIQUE,
            applied_at TIMESTAMPTZ DEFAULT NOW()
        );" 2>/dev/null || true
}

# Function to create database if not exists
create_database_if_not_exists() {
    local db=$1
    
    exists=$(PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -t -c \
        "SELECT 1 FROM pg_database WHERE datname='$db';" 2>/dev/null || echo "0")
    
    if [ "$exists" = "0" ] || [ -z "$exists" ]; then
        echo -e "${YELLOW}Creating database: $db${NC}"
        PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "CREATE DATABASE $db;"
    fi
}

# Main migration process
for db in "${DATABASES[@]}"; do
    echo -e "${GREEN}Migrating database: $db${NC}"
    
    # Create database if not exists
    create_database_if_not_exists $db
    
    # Create migration tracking table
    create_migration_table $db
    
    # Determine which migrations to run based on database
    case $db in
        users_db)
            # Auth and social features go to users_db
            migrations=(
                "001_auth_tables.sql"
                "002_social_features.sql"
            )
            ;;
        tweets_db)
            # Tweet-specific migrations would go here
            migrations=()
            ;;
        notifications_db)
            # Notification-specific migrations would go here
            migrations=()
            ;;
        media_db)
            # Media tables go to media_db
            migrations=(
                "003_media_tables.sql"
            )
            ;;
    esac
    
    # Run migrations for this database
    for migration in "${migrations[@]}"; do
        migration_file="$MIGRATIONS_DIR/$migration"
        if [ -f "$migration_file" ]; then
            run_migration $db $migration_file
        else
            echo -e "  ${RED}Migration file not found: $migration_file${NC}"
        fi
    done
    
    echo ""
done

echo -e "${GREEN}All migrations completed successfully!${NC}"

# Show migration status
echo ""
echo -e "${YELLOW}Migration Status:${NC}"
for db in "${DATABASES[@]}"; do
    echo -e "\n${GREEN}$db:${NC}"
    PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $db -c \
        "SELECT name, applied_at FROM $MIGRATION_TABLE ORDER BY applied_at DESC LIMIT 5;" 2>/dev/null || \
        echo "  No migrations found"
done