# Understanding Docker /proc and /sys Mounting

## The Key Distinction

There are TWO different mounting contexts:

### 1. ❌ Docker Volume Mounts (at container creation)
```bash
# This FAILS - even with --privileged
docker run --privileged -v /my/fake/proc:/proc container_image
```
**Why it fails**: Docker's runtime (runc) validates filesystem types during container creation. It refuses to mount a regular directory as `/proc`.

### 2. ✅ Manual Mounts (inside running container)
```bash
# This WORKS - with --privileged
docker run --privileged container_image bash -c 'mount --bind /mock-proc /proc'
```

## Solution Breakdown

### Step 1: Start Container with Mock Data in Alternative Locations
```bash
docker run --privileged \
    -v ./mock-proc:/mock-proc \    # ✅ Regular mount (works)
    -v ./mock-sys:/mock-sys \      # ✅ Regular mount (works)
    --entrypoint /entrypoint.sh \
    container_image
```

### Step 2: Entrypoint Performs Bind Mounts
```bash
#!/bin/bash
# This runs INSIDE the privileged container
mount --bind /mock-proc /proc  # ✅ Works!
mount --bind /mock-sys /sys    # ✅ Works!
```

## Timeline of Events

1. **Container Creation**
   - Docker sets up the container filesystem
   - Mounts real `/proc` (procfs) and `/sys` (sysfs)
   - Mounts our mock data to `/mock-proc` and `/mock-sys`

2. **Container Starts**
   - Our entrypoint script runs
   - With `--privileged`, we have CAP_SYS_ADMIN
   - We can now bind mount over `/proc` and `/sys`

3. **NPD Script Runs**
   - Reads from `/proc/stat` → Gets our mock data
   - Reads from `/sys/fs/cgroup/cpu.pressure` → Gets our mock data

## Why This Matters

- **Security**: Docker tries to prevent accidental corruption of critical filesystems
- **Flexibility**: `--privileged` gives us the power to override when needed
- **Testing**: We can inject mock data for system-level scripts

## Visual Representation

```
Container Start:
/proc (procfs)         → Real kernel data
/sys (sysfs)          → Real kernel data  
/mock-proc (ext4)     → Our fake data
/mock-sys (ext4)      → Our fake data

After Entrypoint:
/proc (bind mount)    → Points to /mock-proc (our fake data)
/sys (bind mount)     → Points to /mock-sys (our fake data)
/mock-proc (ext4)     → Our fake data (still accessible)
/mock-sys (ext4)      → Our fake data (still accessible)
```
