#!/bin/bash
# HermesX Backup Automation Script
# 
# This script automates backups for Redis and MinIO storage.
# Add to crontab for scheduled execution:
#
#   # Redis RDB snapshot every 5 minutes
#   */5 * * * * /path/to/deploy/scripts/backup.sh redis
#
#   # MinIO skills backup every 6 hours
#   0 */6 * * * /path/to/deploy/scripts/backup.sh minio
#
#   # Full backup daily at 2 AM
#   0 2 * * * /path/to/deploy/scripts/backup.sh full

set -euo pipefail

# Configuration
REDIS_HOST="${REDIS_HOST:-localhost}"
REDIS_PORT="${REDIS_PORT:-6379}"
MINIO_ALIAS="${MINIO_ALIAS:-hermesx}"
MINIO_BUCKET="${MINIO_BUCKET:-skills}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/hermesx}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"

# Logging
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

error() {
    log "ERROR: $*" >&2
    exit 1
}

# Create backup directory if it doesn't exist
ensure_backup_dir() {
    mkdir -p "$BACKUP_DIR/redis" "$BACKUP_DIR/minio"
}

# Redis backup
backup_redis() {
    log "Starting Redis backup..."
    
    # Trigger BGSAVE
    redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" BGSAVE || error "Failed to trigger Redis BGSAVE"
    
    # Wait for BGSAVE to complete
    while [ "$(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" LASTSAVE)" = "$(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" LASTSAVE)" ]; do
        sleep 1
    done
    
    # Copy RDB file
    local timestamp
    timestamp=$(date '+%Y%m%d_%H%M%S')
    local backup_file="$BACKUP_DIR/redis/dump_${timestamp}.rdb"
    
    # Copy from Redis data directory
    redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" --rdb "$backup_file" || error "Failed to copy Redis RDB"
    
    log "Redis backup completed: $backup_file"
}

# MinIO backup
backup_minio() {
    log "Starting MinIO backup..."
    
    local timestamp
    timestamp=$(date '+%Y%m%d_%H%M%S')
    local backup_path="$BACKUP_DIR/minio/${timestamp}"
    
    # Mirror MinIO bucket to local directory
    mc mirror "$MINIO_ALIAS/$MINIO_BUCKET" "$backup_path" --overwrite || error "Failed to mirror MinIO bucket"
    
    log "MinIO backup completed: $backup_path"
}

# Cleanup old backups
cleanup_old_backups() {
    log "Cleaning up backups older than $RETENTION_DAYS days..."
    
    find "$BACKUP_DIR" -type f -mtime "+$RETENTION_DAYS" -delete
    find "$BACKUP_DIR" -type d -empty -delete
    
    log "Cleanup completed"
}

# Full backup
backup_full() {
    log "Starting full backup..."
    backup_redis
    backup_minio
    cleanup_old_backups
    log "Full backup completed"
}

# Main
main() {
    ensure_backup_dir
    
    case "${1:-full}" in
        redis)
            backup_redis
            ;;
        minio)
            backup_minio
            ;;
        full)
            backup_full
            ;;
        cleanup)
            cleanup_old_backups
            ;;
        *)
            echo "Usage: $0 {redis|minio|full|cleanup}"
            exit 1
            ;;
    esac
}

main "$@"
