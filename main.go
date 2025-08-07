package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "sort"
    "text/tabwriter"

    "github.com/moby/moby/api/types/container"
    "github.com/moby/moby/client"
    "github.com/shirou/gopsutil/v3/mem"
)

type ContainerStat struct {
    Name             string
    ID               string
    MemoryUsage      uint64  // Memory in bytes
    MemoryPercentage float64 // Memory usage as a percentage of total system memory
}

// converts bytes to a human-readable string (KB, MB, GB)
func formatBytes(b uint64) string {
    const unit = 1024
    if b < unit {
        return fmt.Sprintf("%d B", b)
    }
    div, exp := int64(unit), 0
    for n := b / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.2f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func main() {
    ctx := context.Background()

    vmem, err := mem.VirtualMemory()
    if err != nil {
        log.Fatalf("❌ Could not get system memory: %v", err)
    }
    totalSystemMemory := vmem.Total

    // connect to the Docker daemon
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        log.Fatalf("❌ Could not connect to Docker daemon: %v", err)
    }
    defer cli.Close()

    log.Println("Docker daemon connection established")

    // get a list of running containers
    containers, err := cli.ContainerList(ctx, container.ListOptions{})
    if err != nil {
        log.Fatalf("❌ Could not list containers: %v", err)
    }

    log.Println("Containers running:", len(containers))

    var stats []ContainerStat
    var totalContainerMemUsage uint64

    for _, cont := range containers {
        // the `false` flag means we get a single stats snapshot, not a stream
        response, err := cli.ContainerStats(ctx, cont.ID, false)
        if err != nil {
            log.Printf("⚠️  Could not get stats for container %s: %v", cont.ID[:12], err)
            continue
        }
        defer response.Body.Close()

        var v *container.StatsResponse
        if err := json.NewDecoder(response.Body).Decode(&v); err != nil {
            log.Printf("⚠️  Could not decode stats for container %s: %v", cont.ID[:12], err);
            continue
        }

        // calculate memory usage
        // memory_stats.usage - memory_stats.stats.cache
        // this gives a more accurate view of the memory used by the application,
        // excluding the file system cache managed by the kernel

        var memUsage uint64
        if cache, ok := v.MemoryStats.Stats["cache"]; ok {
            memUsage = v.MemoryStats.Usage - cache
        } else {
            memUsage = v.MemoryStats.Usage
        }

        totalContainerMemUsage += memUsage

        memPercentage := 0.0
        if totalSystemMemory > 0 {
            memPercentage = float64(memUsage) / float64(totalSystemMemory) * 100.0
        }

        stats = append(stats, ContainerStat{
            Name:             cont.Names[0],
            ID:               cont.ID[:12],
            MemoryUsage:      memUsage,
            MemoryPercentage: memPercentage,
        })
    }

    sort.Slice(stats, func(i, j int) bool {
        return stats[i].MemoryUsage > stats[j].MemoryUsage
    })

    fmt.Println("🐳 Docker Container Memory Usage")
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
    fmt.Fprintln(w, "CONTAINER NAME\tID\tMEMORY USAGE\tMEM %")
    fmt.Fprintln(w, "--------------\t--\t------------\t-----")

    for _, s := range stats {
        fmt.Fprintf(w, "%s\t%s\t%s\t%.2f%%\n", s.Name, s.ID, formatBytes(s.MemoryUsage), s.MemoryPercentage)
    }
    w.Flush()

    fmt.Printf("\n📊 Total Memory Usage (All Containers): %s\n", formatBytes(totalContainerMemUsage))
}
