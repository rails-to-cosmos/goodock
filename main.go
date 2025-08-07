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
)

type ContainerStat struct {
    Name        string
    ID          string
    MemoryUsage uint64 // in bytes
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

    // iterate over containers to get stats for each one
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

        stats = append(stats, ContainerStat{
            Name:        cont.Names[0],
            ID:          cont.ID[:12], // short id for readability
            MemoryUsage: memUsage,
        })
    }

    // sort the slice by memory usage desc
    sort.Slice(stats, func(i, j int) bool {
        return stats[i].MemoryUsage > stats[j].MemoryUsage
    })

    // print the results in a formatted table
    fmt.Println("🐳 Docker Container Memory Usage (Sorted)")
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
    fmt.Fprintln(w, "CONTAINER NAME\tID\tMEMORY USAGE")
    fmt.Fprintln(w, "--------------\t--\t------------")

    for _, s := range stats {
        fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.ID, formatBytes(s.MemoryUsage))
    }
    w.Flush()
}
