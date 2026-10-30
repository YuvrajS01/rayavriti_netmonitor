package backup

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// psql executes meta-commands (\! runs a shell command, \o and \copy write
// files) and superuser SQL such as COPY ... TO PROGRAM, so restoring an
// uploaded file is code execution unless the file is checked first (S9).
// pg_dump --format=plain output only uses COPY data blocks and, since the
// CVE-2025-8714 fix, \restrict/\unrestrict.

var (
	copyFromStdin = regexp.MustCompile(`(?i)^\s*COPY\s.+\sFROM\s+stdin;\s*$`)
	programClause = regexp.MustCompile(`(?i)\b(TO|FROM)\s+PROGRAM\b`)
)

var allowedMetaCommands = map[string]bool{`\restrict`: true, `\unrestrict`: true}

// checkRestoreScript rejects SQL scripts containing psql meta-commands other
// than the ones pg_dump emits, or COPY ... PROGRAM.
func checkRestoreScript(path string) error {
	f, err := os.Open(path) //nolint:gosec // path is validated against BackupDir by the caller
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	inCopy := false
	for line := 1; sc.Scan(); line++ {
		text := sc.Text()
		if inCopy {
			if text == `\.` {
				inCopy = false
			}
			continue
		}
		trimmed := strings.TrimSpace(text)
		if strings.HasPrefix(trimmed, `\`) {
			cmd := strings.Fields(trimmed)[0]
			if !allowedMetaCommands[cmd] {
				return fmt.Errorf("restore rejected: psql meta-command %s on line %d", cmd, line)
			}
			continue
		}
		if programClause.MatchString(text) {
			return fmt.Errorf("restore rejected: COPY ... PROGRAM on line %d", line)
		}
		if copyFromStdin.MatchString(text) {
			inCopy = true
		}
	}
	return sc.Err()
}

func psqlRestoreCommand(ctx context.Context, dsn, path string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "psql", //nolint:gosec // fixed args; the file was checked by checkRestoreScript
		"--no-psqlrc",
		"--single-transaction",
		"--set=ON_ERROR_STOP=on",
		dsnWithoutPassword(dsn),
		"-f", path,
	)
	cmd.Env = pgEnv(dsn)
	return cmd
}

// dsnWithoutPassword removes the password from a URL DSN so it doesn't
// appear in the process list; pgEnv passes it via PGPASSWORD instead.
func dsnWithoutPassword(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.User == nil {
		return dsn
	}
	u.User = url.User(u.User.Username())
	return u.String()
}

func pgEnv(dsn string) []string {
	env := os.Environ()
	if u, err := url.Parse(dsn); err == nil && u.User != nil {
		if pw, ok := u.User.Password(); ok {
			env = append(env, "PGPASSWORD="+pw)
		}
	}
	return env
}
