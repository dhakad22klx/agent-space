package cli

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"

	"justsay-harness/config"
	"justsay-harness/credentials"
)

func runSetup(in lineReader, out io.Writer) error {
	_, _ = fmt.Fprintln(out, "Welcome to JustSay setup.")
	env, err := config.Load()
	if err != nil {
		return err
	}
	if env["GEMINI_API_KEY"] != "" {
		_, _ = fmt.Fprintln(out, "Existing configuration found. Press Enter to keep saved values.")
	}
	fields := []struct {
		key, label, fallback string
		secret               bool
	}{
		{"GEMINI_API_KEY", "Gemini API key", "", true},
		{"GEMINI_MODEL", "Gemini model", "gemini-3.5-flash-lite", false},
	}
	for _, field := range fields {
		if err := setupField(in, env, field.key, field.label, field.fallback, field.secret); err != nil {
			return err
		}
	}
	if strings.TrimSpace(env["GEMINI_API_KEY"]) == "" {
		return errors.New("GEMINI_API_KEY is required; run justsay setup again")
	}
	env["MODEL_PROVIDER"] = "gemini"
	enabled, _ := strconv.ParseBool(env["HITL_ENABLED"])
	defaultAnswer := "y/N"
	if enabled {
		defaultAnswer = "Y/n"
	}
	answer, err := in.read("Enable Human-in-the-Loop (HITL)? ["+defaultAnswer+"]: ", false)
	if err != nil {
		return errors.New("setup cancelled; configuration was not saved")
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "":
	case "y", "yes", "true":
		enabled = true
	case "n", "no", "false":
		enabled = false
	default:
		return errors.New("answer yes or no for HITL; configuration was not saved")
	}
	env["HITL_ENABLED"] = strconv.FormatBool(enabled)
	if enabled {
		if err := setupField(in, env, "HITL_STATE_STORE", "State backend (inmemory/redis)", "inmemory", false); err != nil {
			return err
		}
		env["HITL_STATE_STORE"] = strings.ToLower(strings.TrimSpace(env["HITL_STATE_STORE"]))
		if env["HITL_STATE_STORE"] != "inmemory" && env["HITL_STATE_STORE"] != "redis" {
			return errors.New("state backend must be inmemory or redis; configuration was not saved")
		}
	}
	if enabled && env["HITL_STATE_STORE"] == "redis" {
		_, _ = fmt.Fprintln(out, "Configure Redis to persist paused state.")
		if err := setupField(in, env, "REDIS_ADDR", "Redis database URL (host:port or redis[s]:// URL)", "", true); err != nil {
			return err
		}
		if err := validateRedisAddress(env["REDIS_ADDR"]); err != nil {
			return err
		}
		username := "default"
		if u, err := url.Parse(env["REDIS_ADDR"]); err == nil && u.User != nil && u.User.Username() != "" {
			username = u.User.Username()
		}
		for _, field := range []struct {
			key, label, fallback string
			secret               bool
		}{
			{"REDIS_USERNAME", "Redis username", username, false},
			{"REDIS_PASSWORD", "Redis password (optional)", "", true},
			{"REDIS_KEY_PREFIX", "Redis key prefix", "justsay", false},
		} {
			if err := setupField(in, env, field.key, field.label, field.fallback, field.secret); err != nil {
				return err
			}
		}
		if u, err := url.Parse(env["REDIS_ADDR"]); err == nil && u.User != nil {
			if password, ok := u.User.Password(); ok && env["REDIS_PASSWORD"] == "" {
				env["REDIS_PASSWORD"] = password
			}
			u.User = nil
			env["REDIS_ADDR"] = u.String()
		}
	} else if env["HITL_STATE_STORE"] == "" {
		env["HITL_STATE_STORE"] = "inmemory"
	}
	if env["MOCK_AGENT_CALL"] == "" {
		env["MOCK_AGENT_CALL"] = "false"
	}
	if err := config.SaveSecrets(env); err != nil {
		return err
	}
	if err := config.SaveSettings(env); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "JustSay setup completed. Settings: %s; credentials: %s\nRun: justsay\nOptional integrations: /verify gmail and /verify telegram inside the agent.\n", config.Path(), credentials.Path())
	return err
}

func validateRedisAddress(raw string) error {
	addr := strings.TrimSpace(raw)
	if strings.Contains(addr, "://") {
		u, err := url.Parse(addr)
		if err != nil || (u.Scheme != "redis" && u.Scheme != "rediss") || u.Hostname() == "" {
			return errors.New("redis address must be host:port or a redis:// or rediss:// URL")
		}
		addr = u.Host
		if u.Port() == "" {
			addr = net.JoinHostPort(u.Hostname(), "6379")
		}
		if u.Path != "" && u.Path != "/" {
			if db, err := strconv.Atoi(strings.TrimPrefix(u.Path, "/")); err != nil || db < 0 {
				return errors.New("redis database index must be non-negative")
			}
		}
	}
	host, port, err := net.SplitHostPort(addr)
	n, portErr := strconv.Atoi(port)
	if err != nil || host == "" || portErr != nil || n < 1 || n > 65535 {
		return errors.New("redis address must include a host and valid port")
	}
	return nil
}

func setupField(in lineReader, env map[string]string, key, label, fallback string, secret bool) error {
	current := env[key]
	if current == "" {
		current = fallback
	}
	prompt := label
	if current != "" {
		if secret {
			prompt += " [Enter to keep saved value]"
		} else {
			prompt += " [" + current + "]"
		}
	}
	value, err := in.read(prompt+": ", secret)
	if err != nil {
		return errors.New("setup cancelled; configuration was not saved")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		value = current
	}
	env[key] = value
	return nil
}
