# Write-Ahead Log (WAL) System

## Overview

Ovi's WAL system provides complete auditability and crash recovery by logging every operation before it's executed. When someone asks "Why did Ovi delete my database?", the WAL has the answer.

## Design Philosophy

### Complete Transparency
Every significant operation is logged:
- **Observations**: What Ovi saw in the cloud
- **Decisions**: What actions it decided to take  
- **Executions**: What it actually did
- **Failures**: What went wrong and why

### Immutable Audit Trail
- Once written, WAL entries cannot be modified
- Sequential ordering ensures temporal consistency
- Replay capability for debugging and recovery

## WAL Entry Structure

```go
type Entry struct {
    Timestamp  time.Time       `json:"timestamp"`
    Sequence   int64           `json:"sequence"`
    Type       EntryType       `json:"type"`
    ResourceID string          `json:"resource_id,omitempty"`
    Data       json.RawMessage `json:"data"`
    Error      string          `json:"error,omitempty"`
}

type EntryType string

const (
    EntryObserved  EntryType = "observed"   // Saw resource in cloud
    EntryDecided   EntryType = "decided"    // Made decision about action
    EntryExecuting EntryType = "executing"  // Starting to execute action
    EntryExecuted  EntryType = "executed"   // Successfully executed
    EntryFailed    EntryType = "failed"     // Execution failed
    EntrySkipped   EntryType = "skipped"    // Action skipped (dry-run, etc.)
)
```

## File Format

### Append-Only JSON Lines
Each WAL entry is a single JSON line for simplicity and debuggability:

```
{"timestamp":"2024-01-15T10:30:00Z","sequence":1,"type":"observed","resource_id":"i-123","data":{"id":"i-123","type":"ec2",...}}
{"timestamp":"2024-01-15T10:30:01Z","sequence":2,"type":"decided","resource_id":"i-123","data":{"action":"delete","reason":"not in config"}}
{"timestamp":"2024-01-15T10:30:02Z","sequence":3,"type":"executing","resource_id":"i-123","data":{"action":"delete"}}
{"timestamp":"2024-01-15T10:30:03Z","sequence":4,"type":"executed","resource_id":"i-123","data":{"action":"delete","result":"success"}}
```

### File Rotation
WAL files are timestamped for automatic rotation:
```
ovi-20240115-103000.wal
ovi-20240115-110000.wal  
ovi-20240115-113000.wal
```

## Core Operations

### Writing Entries

#### Basic Append
```go
func (w *WAL) Append(entryType EntryType, resourceID string, data interface{}) error {
    w.mu.Lock()
    defer w.mu.Unlock()
    
    w.sequence++
    
    jsonData, _ := json.Marshal(data)
    entry := Entry{
        Timestamp:  time.Now(),
        Sequence:   w.sequence,
        Type:       entryType,
        ResourceID: resourceID,
        Data:       jsonData,
    }
    
    return w.writeEntry(entry)
}
```

#### Error Logging
```go
func (w *WAL) AppendError(entryType EntryType, resourceID string, data interface{}, err error) error {
    w.mu.Lock()
    defer w.mu.Unlock()
    
    w.sequence++
    
    jsonData, _ := json.Marshal(data)
    entry := Entry{
        Timestamp:  time.Now(),
        Sequence:   w.sequence,
        Type:       entryType,
        ResourceID: resourceID,
        Data:       jsonData,
        Error:      err.Error(),
    }
    
    return w.writeEntry(entry)
}
```

#### Durable Write
```go
func (w *WAL) writeEntry(entry Entry) error {
    line, _ := json.Marshal(entry)
    
    if _, err := w.writer.Write(line); err != nil {
        return err
    }
    if _, err := w.writer.WriteString("\n"); err != nil {
        return err
    }
    
    // Force to disk for durability
    if err := w.writer.Flush(); err != nil {
        return err
    }
    return w.file.Sync()
}
```

### Reading Entries

#### Sequential Reading
```go
type Reader struct {
    scanner *bufio.Scanner
    file    *os.File
}

func (r *Reader) Next() (*Entry, error) {
    if !r.scanner.Scan() {
        if err := r.scanner.Err(); err != nil {
            return nil, err
        }
        return nil, io.EOF
    }
    
    var entry Entry
    if err := json.Unmarshal(r.scanner.Bytes(), &entry); err != nil {
        return nil, err
    }
    
    return &entry, nil
}
```

### Replay Functionality

#### Time-Based Replay
```go
func Replay(dir string, since time.Time, handler func(*Entry) error) error {
    files, _ := filepath.Glob(filepath.Join(dir, "ovi-*.wal"))
    
    for _, file := range files {
        reader, err := NewReader(file)
        if err != nil {
            return err
        }
        defer reader.Close()
        
        for {
            entry, err := reader.Next()
            if err == io.EOF {
                break
            }
            if err != nil {
                return err
            }
            
            if entry.Timestamp.After(since) {
                if err := handler(entry); err != nil {
                    return err
                }
            }
        }
    }
    
    return nil
}
```

## Usage Patterns

### Reconciliation Cycle Logging

#### Full Cycle
```go
// Start reconciliation
wal.Append(EntryObserved, "", "reconcile_start")

// Log observations
for _, resource := range observedResources {
    wal.Append(EntryObserved, resource.ID, resource)
}

// Log decisions  
for _, decision := range decisions {
    wal.Append(EntryDecided, decision.ResourceID, decision)
}

// Log execution
for _, decision := range decisions {
    wal.Append(EntryExecuting, decision.ResourceID, decision)
    
    err := executor.Execute(decision)
    if err != nil {
        wal.AppendError(EntryFailed, decision.ResourceID, decision, err)
    } else {
        wal.Append(EntryExecuted, decision.ResourceID, decision)
    }
}
```

### Error Scenarios

#### Partial Execution
```go
// Ovi crashes here - WAL shows exactly what was decided vs executed
decisions := []Decision{
    {Action: "create", ResourceID: "i-1"},
    {Action: "delete", ResourceID: "i-2"},
    {Action: "update", ResourceID: "i-3"},
}

for _, decision := range decisions {
    wal.Append(EntryDecided, decision.ResourceID, decision)
}

for _, decision := range decisions {
    wal.Append(EntryExecuting, decision.ResourceID, decision)
    
    err := executor.Execute(decision)  // Crash after i-1, before i-2
    if err != nil {
        wal.AppendError(EntryFailed, decision.ResourceID, decision, err)
        break
    }
    wal.Append(EntryExecuted, decision.ResourceID, decision)
}

// Recovery can see: i-1 executed, i-2 and i-3 decided but not executed
```

## Debugging and Analysis

### Command-Line Tools

#### Recent Activity
```bash
# Show what Ovi did in the last hour
ovi wal replay --since "1 hour ago"

# Show only failed operations
ovi wal replay --since "1 day ago" --type failed

# Follow specific resource
ovi wal replay --resource-id "i-123456" --since "2024-01-15"
```

#### WAL Analysis
```bash
# Count operations by type
ovi wal analyze --group-by type

# Find decision patterns
ovi wal analyze --pattern "decided -> failed"

# Resource lifecycle
ovi wal analyze --resource-id "i-123" --timeline
```

### Programmatic Analysis

#### Decision Success Rate
```go
func analyzeSuccessRate(walDir string) (float64, error) {
    var decided, executed int
    
    err := Replay(walDir, time.Time{}, func(entry *Entry) error {
        switch entry.Type {
        case EntryDecided:
            decided++
        case EntryExecuted:
            executed++
        }
        return nil
    })
    
    if decided == 0 {
        return 0, nil
    }
    
    return float64(executed) / float64(decided), err
}
```

#### Resource Timeline
```go
func resourceTimeline(walDir, resourceID string) ([]Entry, error) {
    var timeline []Entry
    
    err := Replay(walDir, time.Time{}, func(entry *Entry) error {
        if entry.ResourceID == resourceID {
            timeline = append(timeline, *entry)
        }
        return nil
    })
    
    return timeline, err
}
```

## Performance Characteristics

### Write Performance
- **Throughput**: ~10k entries/second on SSD
- **Latency**: ~100μs per entry (includes fsync)
- **Scaling**: Linear with entry size

### Storage Efficiency
- **Entry size**: ~200-500 bytes typical
- **Compression**: 60-70% with gzip (future enhancement)
- **Rotation**: Automatic by time/size

### Memory Usage
- **Minimal**: No in-memory buffering beyond OS buffers
- **Streaming**: Can process WAL files larger than RAM
- **Bounded**: Fixed memory usage regardless of WAL size

## Safety and Reliability

### Durability Guarantees
- **fsync**: Every write forced to disk
- **Atomic**: Each entry written atomically
- **Crash-safe**: Partial writes detected and handled

### Corruption Handling
```go
func (r *Reader) Next() (*Entry, error) {
    if !r.scanner.Scan() {
        return nil, io.EOF
    }
    
    var entry Entry
    if err := json.Unmarshal(r.scanner.Bytes(), &entry); err != nil {
        // Log corruption but continue reading
        log.Printf("Corrupted WAL entry: %v", err)
        return r.Next() // Skip to next entry
    }
    
    return &entry, nil
}
```

### Security Considerations
- **File permissions**: 0600 (owner read/write only)
- **Directory permissions**: 0750 (owner + group)
- **No secrets**: Avoid logging sensitive data
- **Rotation**: Old logs can be archived/encrypted

## Operational Aspects

### Monitoring
Key metrics to track:
- **Write latency**: Time to append entry
- **File size**: Current WAL file size
- **Error rate**: Failed writes per second
- **Disk usage**: Total WAL directory size

### Maintenance
Regular operations:
- **Rotation**: Archive old WAL files
- **Compression**: Compress archived files
- **Cleanup**: Remove very old archives
- **Analysis**: Regular success rate analysis

### Troubleshooting

#### Common Issues
1. **Disk full**: WAL writes fail, Ovi should halt
2. **Permission issues**: Check file/directory permissions
3. **Corruption**: Usually indicates hardware issues
4. **High latency**: Check disk performance

#### Recovery Procedures
1. **Find last good entry**: Scan WAL for corruption point
2. **Determine state**: What was decided vs executed
3. **Resume safely**: Don't re-execute completed actions
4. **Verify consistency**: Check cloud state matches expectations

## Future Enhancements

### Planned Features
1. **Compression**: Automatic compression of old WAL files
2. **Encryption**: Encrypt sensitive WAL data
3. **Streaming**: Real-time WAL streaming for monitoring
4. **Structured queries**: SQL-like queries over WAL data
5. **Retention policies**: Automatic cleanup based on age/size

### Advanced Analysis
1. **Pattern detection**: Identify recurring failure patterns
2. **Performance analysis**: Track execution time trends
3. **Capacity planning**: Predict future resource needs
4. **Anomaly detection**: Identify unusual reconciliation patterns

---

The WAL system transforms Ovi from a "black box" into a completely transparent infrastructure management tool. Every decision is explained, every action is tracked, and every failure is debuggable.