package engine

import (
	"os"
	"runtime"
	"strings"
)

// GetCommandEnv returns the current process environment, ensuring essential variables
// like HOME, USER, and PATH are always populated so agy never crashes with "$HOME is not defined".
func GetCommandEnv() []string {
	env := os.Environ()
	hasHome := false
	hasUser := false

	for _, e := range env {
		if strings.HasPrefix(e, "HOME=") && len(e) > 5 {
			hasHome = true
		}
		if strings.HasPrefix(e, "USER=") && len(e) > 5 {
			hasUser = true
		}
	}

	if !hasHome {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			if runtime.GOOS == "windows" {
				home = os.Getenv("USERPROFILE")
				if home == "" {
					home = `C:\Users\Default`
				}
			} else {
				home = os.Getenv("HOME")
				if home == "" {
					home = "/root"
				}
			}
		}
		env = append(env, "HOME="+home)
	}

	if !hasUser {
		user := os.Getenv("USER")
		if user == "" {
			user = "root"
		}
		env = append(env, "USER="+user)
	}

	return env
}
