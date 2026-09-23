package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/FullFran/bifrost/internal/realm"
	"github.com/FullFran/bifrost/internal/workspace"
)

type JobStatus string

const (
	StatusRunning   JobStatus = "RUNNING"
	StatusCompleted JobStatus = "COMPLETED"
	StatusFailed    JobStatus = "FAILED"
	StatusAborted   JobStatus = "ABORTED"
)

type Job struct {
	ID             string     `json:"id"`
	Realm          string     `json:"realm"`
	Prompt         string     `json:"prompt"`
	WorkDir        string     `json:"work_dir"`
	WorktreeBranch string     `json:"worktree_branch,omitempty"`
	WorktreePath   string     `json:"worktree_path,omitempty"`
	RepoRoot       string     `json:"repo_root,omitempty"`
	PID            int        `json:"pid"`
	Status         JobStatus  `json:"status"`
	ExitCode       int        `json:"exit_code"`
	CreatedAt      time.Time  `json:"created_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	LogFile        string     `json:"log_file"`
	Error          string     `json:"error,omitempty"`
}

func CacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cache", "bifrost")
	if err := os.MkdirAll(filepath.Join(dir, "jobs"), 0755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func jobFilePath(id string) (string, error) {
	cache, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "jobs", fmt.Sprintf("%s.json", id)), nil
}

func logFilePath(id string) (string, error) {
	cache, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "logs", fmt.Sprintf("%s.log", id)), nil
}

func SaveJob(j *Job) error {
	p, err := jobFilePath(j.ID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

func GetJob(id string) (*Job, error) {
	p, err := jobFilePath(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("job '%s' not found: %w", id, err)
	}
	var j Job
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, err
	}

	// Dynamic check if process is still alive when status is RUNNING
	if j.Status == StatusRunning && j.PID > 0 {
		if !isProcessAlive(j.PID) {
			// Process died without updating status
			j.Status = StatusFailed
			j.Error = "process exited unexpectedly"
			now := time.Now()
			j.FinishedAt = &now
			_ = SaveJob(&j)
		}
	}

	return &j, nil
}

func ListJobs() ([]*Job, error) {
	cache, err := CacheDir()
	if err != nil {
		return nil, err
	}
	files, err := os.ReadDir(filepath.Join(cache, "jobs"))
	if err != nil {
		return nil, err
	}

	var jobs []*Job
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			id := f.Name()[:len(f.Name())-5]
			j, err := GetJob(id)
			if err == nil {
				jobs = append(jobs, j)
			}
		}
	}

	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})

	return jobs, nil
}

func AbortJob(id string) error {
	j, err := GetJob(id)
	if err != nil {
		return err
	}
	if j.Status != StatusRunning {
		return fmt.Errorf("job is already in state %s", j.Status)
	}

	killProcessGroup(j.PID)

	j.Status = StatusAborted
	now := time.Now()
	j.FinishedAt = &now
	j.Error = "aborted by user"
	return SaveJob(j)
}

// RunForeground executes the agent synchronously in the console.
func RunForeground(ctx context.Context, task realm.Task, r realm.Realm, targetDir string, wt *workspace.WorktreeSession) error {
	cmd, err := r.BuildCommand(ctx, task, targetDir)
	if err != nil {
		return err
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// SpawnBackground creates a detached background worker process.
func SpawnBackground(task realm.Task, r realm.Realm, targetDir string, wt *workspace.WorktreeSession) (*Job, error) {
	logPath, err := logFilePath(task.ID)
	if err != nil {
		return nil, err
	}

	selfBin, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to locate bifrost executable: %w", err)
	}

	job := &Job{
		ID:        task.ID,
		Realm:     r.ID(),
		Prompt:    task.Prompt,
		WorkDir:   targetDir,
		Status:    StatusRunning,
		CreatedAt: time.Now(),
		LogFile:   logPath,
	}

	if wt != nil {
		job.WorktreeBranch = wt.Branch
		job.WorktreePath = wt.Path
		job.RepoRoot = wt.RepoRoot
	}

	if err := SaveJob(job); err != nil {
		return nil, err
	}

	// Worker process: bifrost __worker <job-id>
	workerCmd := exec.Command(selfBin, "__worker", task.ID)
	setSysProcAttr(workerCmd)

	if err := workerCmd.Start(); err != nil {
		job.Status = StatusFailed
		job.Error = err.Error()
		_ = SaveJob(job)
		return nil, fmt.Errorf("failed to start worker: %w", err)
	}

	job.PID = workerCmd.Process.Pid
	if err := SaveJob(job); err != nil {
		return nil, err
	}

	return job, nil
}

// ExecuteWorker is called by the internal '__worker' hidden subcommand.
func ExecuteWorker(jobID string) error {
	j, err := GetJob(jobID)
	if err != nil {
		return err
	}

	logFile, err := os.OpenFile(j.LogFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		j.Status = StatusFailed
		j.Error = fmt.Sprintf("failed to open log file: %v", err)
		_ = SaveJob(j)
		return err
	}
	defer func() { _ = logFile.Close() }()

	r, ok := realm.Get(j.Realm)
	if !ok {
		j.Status = StatusFailed
		j.Error = fmt.Sprintf("realm %s not found", j.Realm)
		_ = SaveJob(j)
		return fmt.Errorf("realm not found")
	}

	task := realm.Task{
		ID:      j.ID,
		Prompt:  j.Prompt,
		WorkDir: j.WorkDir,
	}

	ctx := context.Background()
	cmd, err := r.BuildCommand(ctx, task, j.WorkDir)
	if err != nil {
		j.Status = StatusFailed
		j.Error = err.Error()
		_ = SaveJob(j)
		return err
	}

	// Stream stdout & stderr to logFile
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	runErr := cmd.Run()
	now := time.Now()
	j.FinishedAt = &now

	if runErr != nil {
		j.Status = StatusFailed
		j.Error = runErr.Error()
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			j.ExitCode = exitErr.ExitCode()
		} else {
			j.ExitCode = 1
		}
	} else {
		j.Status = StatusCompleted
		j.ExitCode = 0
	}

	return SaveJob(j)
}

// PrintLog displays the log of a given job.
func PrintLog(id string, tailLines int) error {
	j, err := GetJob(id)
	if err != nil {
		return err
	}

	f, err := os.Open(j.LogFile)
	if err != nil {
		return fmt.Errorf("could not open log file: %w", err)
	}
	defer func() { _ = f.Close() }()

	_, err = io.Copy(os.Stdout, f)
	return err
}

// FollowLog continuously streams logs from a running job until finished or context cancelled.
func FollowLog(ctx context.Context, id string, out io.Writer) error {
	j, err := GetJob(id)
	if err != nil {
		return err
	}

	f, err := os.Open(j.LogFile)
	if err != nil {
		return fmt.Errorf("could not open log file: %w", err)
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			n, err := f.Read(buf)
			if n > 0 {
				_, _ = out.Write(buf[:n])
			}
			if err == io.EOF || n == 0 {
				// Refresh job status to see if it's finished
				current, err := GetJob(id)
				if err == nil && current.Status != StatusRunning {
					// Read any remaining bytes
					for {
						n, err := f.Read(buf)
						if n > 0 {
							_, _ = out.Write(buf[:n])
						}
						if err != nil || n == 0 {
							break
						}
					}
					return nil
				}
				time.Sleep(200 * time.Millisecond)
				continue
			}
			if err != nil {
				return err
			}
		}
	}
}

// CleanJob removes worktrees and optionally deletes associated git branch.
func CleanJob(id string, deleteBranch bool) error {
	j, err := GetJob(id)
	if err != nil {
		return err
	}

	if j.Status == StatusRunning {
		return fmt.Errorf("cannot clean job '%s' while it is still RUNNING (abort it first)", id)
	}

	if j.WorktreePath != "" {
		_ = workspace.Cleanup(context.Background(), j.RepoRoot, j.WorktreePath, j.WorktreeBranch, deleteBranch)
		j.WorktreePath = ""
		if deleteBranch {
			j.WorktreeBranch = ""
		}
		_ = SaveJob(j)
	}

	return nil
}

// PruneJobs removes finished jobs, deletes associated worktrees, and cleans log files.
func PruneJobs(all bool) (int, error) {
	jobs, err := ListJobs()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, j := range jobs {
		if j.Status == StatusRunning {
			continue
		}
		// Clean worktree
		if j.WorktreePath != "" {
			_ = workspace.Cleanup(context.Background(), j.RepoRoot, j.WorktreePath, j.WorktreeBranch, true)
		}
		// Remove log file
		if j.LogFile != "" {
			_ = os.Remove(j.LogFile)
		}
		// Remove job json file
		jobPath, err := jobFilePath(j.ID)
		if err == nil {
			_ = os.Remove(jobPath)
		}
		count++
	}

	return count, nil
}
