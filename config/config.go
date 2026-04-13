package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config 统一的 Agent Runtime 配置
type Config struct {
	Runtime      RuntimeConfig      `json:"runtime"`
	Worker       WorkerConfig       `json:"worker"`
	Queue        QueueConfig        `json:"queue"`
	Tracer       TracerConfig       `json:"tracer"`
	Distributed  DistributedConfig  `json:"distributed"`
}

// RuntimeConfig runtime.Engine 配置
type RuntimeConfig struct {
	CheckpointType     string        `json:"checkpoint_type"`
	CheckpointURL      string        `json:"checkpoint_url"`
	TaskTimeout        time.Duration `json:"task_timeout"`
	MaxConcurrentTasks int           `json:"max_concurrent_tasks"`
}

// WorkerConfig worker 配置
type WorkerConfig struct {
	Concurrency     int           `json:"concurrency"`
	Retry           RetryConfig   `json:"retry"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// RetryConfig 重试策略配置
type RetryConfig struct {
	MaxRetries int           `json:"max_retries"`
	BaseDelay  time.Duration `json:"base_delay"`
	MaxDelay   time.Duration `json:"max_delay"`
	Factor     float64       `json:"factor"`
}

// QueueConfig 队列配置
type QueueConfig struct {
	BackendType  string        `json:"backend_type"`
	BackendURL   string        `json:"backend_url"`
	QueueSize    int           `json:"queue_size"`
	BatchSize    int           `json:"batch_size"`
	BatchTimeout time.Duration `json:"batch_timeout"`
}

// TracerConfig tracer 配置
type TracerConfig struct {
	Type         string  `json:"type"`
	Endpoint     string  `json:"endpoint"`
	SamplingRate float64 `json:"sampling_rate"`
	ServiceName  string  `json:"service_name"`
}

// DistributedConfig 分布式配置
type DistributedConfig struct {
	Enabled              bool   `json:"enabled"`
	NodeName             string `json:"node_name"`
	NodeID               string `json:"node_id"`
	MessageBroker        string `json:"message_broker"`
	BrokerURL            string `json:"broker_url"`
	ServiceRegistry      string `json:"service_registry"`
	RegistryURL          string `json:"registry_url"`
	EnableLeaderElection bool   `json:"enable_leader_election"`
	LeaderElectionKey    string `json:"leader_election_key"`
}

// Default 返回默认配置
func Default() *Config {
	return &Config{
		Runtime: RuntimeConfig{
			CheckpointType:     "noop",
			TaskTimeout:        30 * time.Second,
			MaxConcurrentTasks: 100,
		},
		Worker: WorkerConfig{
			Concurrency:     4,
			ShutdownTimeout: 30 * time.Second,
			Retry: RetryConfig{
				MaxRetries: 3,
				BaseDelay:  1 * time.Second,
				MaxDelay:   30 * time.Second,
				Factor:     2.0,
			},
		},
		Queue: QueueConfig{
			BackendType:  "memory",
			QueueSize:    1000,
			BatchSize:    10,
			BatchTimeout: 5 * time.Second,
		},
		Tracer: TracerConfig{
			Type:         "memory",
			SamplingRate: 1.0,
			ServiceName:  "agent-runtime",
		},
		Distributed: DistributedConfig{
			Enabled: false,
		},
	}
}

// FromEnv 从环境变量加载配置
func FromEnv() *Config {
	cfg := Default()

	// Runtime 配置
	if v := os.Getenv("AGENT_CHECKPOINT_TYPE"); v != "" {
		cfg.Runtime.CheckpointType = v
	}
	if v := os.Getenv("AGENT_CHECKPOINT_URL"); v != "" {
		cfg.Runtime.CheckpointURL = v
	}
	if v := os.Getenv("AGENT_TASK_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Runtime.TaskTimeout = d
		}
	}

	// Worker 配置
	if v := os.Getenv("AGENT_WORKER_CONCURRENCY"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Worker.Concurrency)
	}
	if v := os.Getenv("AGENT_RETRY_MAX_RETRIES"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Worker.Retry.MaxRetries)
	}
	if v := os.Getenv("AGENT_RETRY_BASE_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Worker.Retry.BaseDelay = d
		}
	}

	// Queue 配置
	if v := os.Getenv("AGENT_QUEUE_BACKEND"); v != "" {
		cfg.Queue.BackendType = v
	}
	if v := os.Getenv("AGENT_QUEUE_URL"); v != "" {
		cfg.Queue.BackendURL = v
	}

	// Tracer 配置
	if v := os.Getenv("AGENT_TRACER_TYPE"); v != "" {
		cfg.Tracer.Type = v
	}
	if v := os.Getenv("AGENT_TRACER_ENDPOINT"); v != "" {
		cfg.Tracer.Endpoint = v
	}
	if v := os.Getenv("AGENT_TRACER_SERVICE_NAME"); v != "" {
		cfg.Tracer.ServiceName = v
	}

	// 分布式配置
	if v := os.Getenv("AGENT_DISTRIBUTED_ENABLED"); v == "true" {
		cfg.Distributed.Enabled = true
	}
	if v := os.Getenv("AGENT_NODE_NAME"); v != "" {
		cfg.Distributed.NodeName = v
	}

	return cfg
}

// FromJSON 从 JSON 加载配置
func FromJSON(data []byte) (*Config, error) {
	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ToJSON 将配置转换为 JSON
func (c *Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Worker.Concurrency <= 0 {
		return fmt.Errorf("worker concurrency must be > 0")
	}
	if c.Queue.QueueSize <= 0 {
		return fmt.Errorf("queue size must be > 0")
	}
	if c.Worker.Retry.MaxRetries < 0 {
		return fmt.Errorf("max retries must be >= 0")
	}
	if c.Tracer.SamplingRate < 0 || c.Tracer.SamplingRate > 1 {
		return fmt.Errorf("sampling rate must be between 0 and 1")
	}
	return nil
}

// Clone 克隆配置
func (c *Config) Clone() *Config {
	cfg := *c
	return &cfg
}
