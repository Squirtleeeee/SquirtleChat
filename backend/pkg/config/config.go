package config



import (

	"bufio"

	"os"

	"path/filepath"

	"strconv"

	"strings"

)



type Config struct {

	HTTPPort          string

	WSPort            string

	MySQLDSN          string

	RedisAddr         string

	KafkaBroker       string

	JWTSecret         string

	MinioEndpoint     string

	MinioAccessKey    string

	MinioSecretKey    string

	MinioBucket       string

	GatewayInstanceID string

	LLMAPIBase        string

	LLMAPIKey         string

	LLMModel          string

}



func Load() *Config {

	loadOptionalEnvFiles()

	cfg := &Config{

		HTTPPort:          getEnv("HTTP_PORT", "8080"),

		WSPort:            getEnv("WS_PORT", "8081"),

		MySQLDSN:          getEnv("MYSQL_DSN", "squirtle:squirtle123@tcp(localhost:3306)/squirtlechat?parseTime=true&loc=Local"),

		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),

		KafkaBroker:       getEnv("KAFKA_BROKER", "localhost:29092"),

		JWTSecret:         getEnv("JWT_SECRET", "squirtlechat-dev-secret-change-in-prod"),

		MinioEndpoint:     getEnv("MINIO_ENDPOINT", "localhost:9000"),

		MinioAccessKey:    getEnv("MINIO_ACCESS_KEY", "minioadmin"),

		MinioSecretKey:    getEnv("MINIO_SECRET_KEY", "minioadmin123"),

		MinioBucket:       getEnv("MINIO_BUCKET", "squirtlechat"),

		GatewayInstanceID: getEnv("GATEWAY_INSTANCE_ID", "gw-1"),

		LLMAPIBase:        getEnv("LLM_API_BASE", "https://api.openai.com/v1"),

		LLMAPIKey:         getEnv("LLM_API_KEY", ""),

		LLMModel:          getEnv("LLM_MODEL", "gpt-4o-mini"),

	}

	normalizeLLMConfig(cfg)

	return cfg

}



func getEnv(key, def string) string {

	if v := os.Getenv(key); v != "" {

		return v

	}

	return def

}



// loadOptionalEnvFiles loads deploy/llm.env so gateways pick up LLM settings without shell exports.

func loadOptionalEnvFiles() {

	seen := map[string]struct{}{}

	add := func(paths ...string) {

		for _, path := range paths {

			if path == "" {

				continue

			}

			if abs, err := filepath.Abs(path); err == nil {

				path = abs

			}

			if _, ok := seen[path]; ok {

				continue

			}

			seen[path] = struct{}{}

			force := strings.HasSuffix(strings.ToLower(filepath.Base(path)), "llm.env")

			loadEnvFile(path, force)

		}

	}

	if p := os.Getenv("LLM_ENV_FILE"); p != "" {

		add(p)

	}

	if wd, err := os.Getwd(); err == nil {

		dir := wd

		for i := 0; i < 6; i++ {

			add(filepath.Join(dir, "deploy", "llm.env"))

			parent := filepath.Dir(dir)

			if parent == dir {

				break

			}

			dir = parent

		}

	}

	if exe, err := os.Executable(); err == nil {

		dir := filepath.Dir(exe)

		for i := 0; i < 6; i++ {

			add(filepath.Join(dir, "deploy", "llm.env"))

			parent := filepath.Dir(dir)

			if parent == dir {

				break

			}

			dir = parent

		}

	}

}



func loadEnvFile(path string, forceLLM bool) {

	f, err := os.Open(path)

	if err != nil {

		return

	}

	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {

		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {

			continue

		}

		i := strings.IndexByte(line, '=')

		if i <= 0 {

			continue

		}

		key := strings.TrimSpace(line[:i])

		val := strings.TrimSpace(line[i+1:])

		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {

			val = val[1 : len(val)-1]

		}

		if key == "" {

			continue

		}

		if forceLLM && strings.HasPrefix(key, "LLM_") {

			_ = os.Setenv(key, val)

			continue

		}

		if os.Getenv(key) == "" {

			_ = os.Setenv(key, val)

		}

	}

}



// normalizeLLMConfig fixes common DeepSeek misconfiguration (OpenAI base + DeepSeek key).

func normalizeLLMConfig(cfg *Config) {

	if cfg.LLMAPIKey == "" {

		return

	}

	base := strings.TrimRight(cfg.LLMAPIBase, "/")

	model := strings.ToLower(cfg.LLMModel)

	isDeepSeek := strings.Contains(model, "deepseek") ||

		strings.Contains(base, "deepseek.com") ||

		strings.Contains(strings.ToLower(cfg.LLMAPIKey), "deepseek")



	if isDeepSeek {

		if base == "" || strings.Contains(base, "openai.com") {

			cfg.LLMAPIBase = "https://api.deepseek.com"

		} else if strings.Contains(base, "deepseek.com") && strings.HasSuffix(base, "/v1") {

			cfg.LLMAPIBase = strings.TrimSuffix(base, "/v1")

		} else {

			cfg.LLMAPIBase = base

		}

		if cfg.LLMModel == "" || cfg.LLMModel == "gpt-4o-mini" {

			cfg.LLMModel = "deepseek-v4-flash"

		}

		_ = os.Setenv("LLM_API_BASE", cfg.LLMAPIBase)

		_ = os.Setenv("LLM_MODEL", cfg.LLMModel)

	}

}



func GetEnvInt(key string, def int) int {

	if v := os.Getenv(key); v != "" {

		if i, err := strconv.Atoi(v); err == nil {

			return i

		}

	}

	return def

}


