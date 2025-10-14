#!/bin/bash

# Common helper functions for Node Problem Detector plugins
# This file contains shared code used by all NPD plugins for basic logging and utilities


# Log directory
LOG_DIR="/var/log/azure/Microsoft.AKS.Compute.AKS.Linux.AKSNode/events"
# Max message length for Telemetry events
MAX_MESSAGE_LENGTH=3072   

# Array to collect all debug messages
LOG_MESSAGES=()
# Function to generate a UUID
generate_guid() {
    # Check if uuidgen is available (most Linux systems)
    if command -v uuidgen >/dev/null 2>&1; then
        uuidgen
        return
    fi
    
    # Fallback method if uuidgen is not available
    # This creates a UUID v4 format using /dev/urandom
    local hex
    hex=$(od -x /dev/urandom | head -1 | awk '{OFS="-"; print $2$3,$4,$5,$6,$7$8$9}')
    echo "${hex:0:8}-${hex:9:4}-4${hex:14:3}-${hex:18:4}-${hex:23:12}"
}

# use same operation id for all records in a session so can correlate and concat chunked log messages
OPERATION_ID=$(generate_guid)

# Function to redact sensitive information from command lines and log output
redact_sensitive_data() {
    local input="$1"
    echo "$input" | sed -E \
        -e 's/(password|passwd|pwd)([=:])[^[:space:]]*/\1\2***REDACTED***/gi' \
        -e 's/(token|auth_token|access_token)([=:])[^[:space:]]*/\1\2***REDACTED***/gi' \
        -e 's/(key|api_key|secret_key|private_key)([=:])[^[:space:]]*/\1\2***REDACTED***/gi' \
        -e 's/(secret|auth_secret|client_secret)([=:])[^[:space:]]*/\1\2***REDACTED***/gi' \
        -e 's/(auth|authorization)([=:])[^[:space:]]*/\1\2***REDACTED***/gi' \
        -e 's/(-D[^[:space:]]*password[^[:space:]]*)([=:])[^[:space:]]*/\1\2***REDACTED***/gi' \
        -e 's/(-D[^[:space:]]*token[^[:space:]]*)([=:])[^[:space:]]*/\1\2***REDACTED***/gi' \
        -e 's/(-D[^[:space:]]*key[^[:space:]]*)([=:])[^[:space:]]*/\1\2***REDACTED***/gi' \
        -e 's/(-D[^[:space:]]*secret[^[:space:]]*)([=:])[^[:space:]]*/\1\2***REDACTED***/gi' \
        -e 's/(--[^[:space:]]*token[^[:space:]]*)([=:])[^[:space:]]*/\1\2***REDACTED***/gi' \
        -e 's/(--[^[:space:]]*password[^[:space:]]*)([=:])[^[:space:]]*/\1\2***REDACTED***/gi' \
        -e 's/(--[^[:space:]]*key[^[:space:]]*)([=:])[^[:space:]]*/\1\2***REDACTED***/gi' \
        -e 's/(--[^[:space:]]*secret[^[:space:]]*)([=:])[^[:space:]]*/\1\2***REDACTED***/gi'
}

# Function to log debug messages
log() {
    local message="$1"
    local suppress_echo=false
    
    # Check if second argument is --no-echo
    if [[ "${2:-}" == "--no-echo" ]]; then
        suppress_echo=true
    fi
    
    # Always add to LOG_MESSAGES array
    LOG_MESSAGES+=("$message")
    
    # Only echo if not suppressed
    if [[ "$suppress_echo" != true ]]; then
        echo "$message"
    fi
}

# Function to clean up old log files
cleanup_old_logs() {
    if [ -d "$LOG_DIR" ]; then
        find $LOG_DIR -mmin +10 -type f -delete 2>/dev/null || log "WARNING: Failed to clean up old log files"
    else
        log "WARNING: Log directory $LOG_DIR does not exist or is not accessible"
    fi
}


# Function to write debug logs to file
write_logs() {
    local taskname="$1"  # Pass in the specific taskname (e.g., "npd:check_cpu_pressure:log")
    
    # ISO-8601 format required
    TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    UNIX_TIMESTAMP=$(date +%s%3N)
    
    DEBUG_LOG=$(printf "%s\n" "${LOG_MESSAGES[@]}")
    
    # Redact sensitive information from debug logs
    DEBUG_LOG=$(redact_sensitive_data "$DEBUG_LOG")
    
    JSON_LOG=$(jq -n --arg timestamp "$TIMESTAMP" \
                  --arg taskname "$taskname" \
                  --arg eventlevel "Warning" \
                  --arg message "$DEBUG_LOG" \
                  --arg eventpid "$$" \
                  --arg eventtid "1" \
                  --arg opid "$OPERATION_ID" \
                  '{
                    Version: "1.0",
                    Timestamp: $timestamp,
                    TaskName: $taskname,
                    EventLevel: $eventlevel,
                    Message: $message,
                    EventPid: $eventpid,
                    EventTid: $eventtid,
                    OperationId: $opid
                  }')
    
    # Filename is required to be named with the Unix timestamp
    DEBUG_LOG_FILE="$LOG_DIR/${UNIX_TIMESTAMP}.json"
    echo "$JSON_LOG" > "$DEBUG_LOG_FILE"
}

# Function to write the telemetry event log file
write_event_log() {
    local taskname="$1"   # Task name (e.g., "npd:check_node_not_ready:route_info")
    local message="$2"    # Message content to be logged
    
        # ISO-8601 format required
    TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    UNIX_TIMESTAMP=$(date +%s%3N)
    
    JSON_LOG=$(jq -n \
                --arg timestamp "$TIMESTAMP" \
                --arg taskname "$taskname" \
                --arg message "$message" \
                --arg eventpid "$$" \
                --arg eventtid "1" \
                --arg opid "$OPERATION_ID" \
                '{
                  Version: "1.0",
                  Timestamp: $timestamp,
                  TaskName: $taskname,
                  EventLevel: "Warning",
                  Message: $message,
                  EventPid: $eventpid,
                  EventTid: $eventtid,
                  OperationId: $opid
                }')
    
    LOG_FILE="$LOG_DIR/$UNIX_TIMESTAMP.json"
    echo "$JSON_LOG" > "$LOG_FILE"    
}

# Function to write large log content by splitting it into multiple chunks
# This handles content larger than MAX_MESSAGE_LENGTH
write_chunked_event_log() {
    local base_taskname="$1"    # Base task name (e.g., "npd:check_node_not_ready:route_info")
    local content="$2"          # Content to be split and logged
    local max_chunks="${3:-10}" # Maximum number of chunks to write (default: 10)
    
    # Check if content is smaller than MAX_MESSAGE_LENGTH
    if [ ${#content} -le "$MAX_MESSAGE_LENGTH" ]; then
        # If small enough, just write as a single log
        write_event_log "$base_taskname" "$content"
        return
    fi
    
    log "${base_taskname} output is large (${#content} chars), splitting into chunks (max: $max_chunks)..." --no-echo
    
    # Convert the content to an array of lines
    mapfile -t CONTENT_LINES <<< "$content"
    
    TOTAL_LINES=${#CONTENT_LINES[@]}
    log "Total lines to split: $TOTAL_LINES" --no-echo
    
    CHUNK_START=0
    CHUNK_NUM=1
    CURRENT_CHUNK=""
    
    while [ "$CHUNK_START" -lt "$TOTAL_LINES" ] && [ "$CHUNK_NUM" -le "$max_chunks" ]; do
        # Reset chunk content
        CURRENT_CHUNK=""
        LINES_ADDED=0
        
        # Build up the chunk until we approach the max size
        for (( i=CHUNK_START; i<TOTAL_LINES; i++ )); do
            LINE="${CONTENT_LINES[$i]}"
            # Calculate size if we add this line
            NEW_CHUNK="$CURRENT_CHUNK$LINE"$'\n'
            NEW_SIZE=${#NEW_CHUNK}
            
            # If adding this line would exceed max size and we already have content, stop
            if [ "$NEW_SIZE" -gt $((MAX_MESSAGE_LENGTH - 200)) ] && [ -n "$CURRENT_CHUNK" ]; then
                break
            fi
            
            # Add line to current chunk
            CURRENT_CHUNK="$NEW_CHUNK"
            LINES_ADDED=$((LINES_ADDED + 1))
        done
        
        # Update chunk start for next iteration
        CHUNK_START=$((CHUNK_START + LINES_ADDED))
        
        write_event_log "${base_taskname}_${CHUNK_NUM}" "$CURRENT_CHUNK"
        
        CHUNK_NUM=$((CHUNK_NUM + 1))
        
        # Check if we're about to hit the max chunks limit
        if [ "$CHUNK_NUM" -eq "$max_chunks" ] && [ "$CHUNK_START" -lt "$TOTAL_LINES" ]; then
            log "WARNING: Reached maximum chunk limit ($max_chunks). Truncating remaining $((TOTAL_LINES - CHUNK_START)) lines." --no-echo
        fi
    done
    
    log "Completed writing $((CHUNK_NUM - 1)) chunks for ${base_taskname}" --no-echo
} 
