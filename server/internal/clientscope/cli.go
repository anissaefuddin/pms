package clientscope

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Run executes "dry-run" or "apply" with --mapping FILE. Apply also takes --expect-digest from the reviewed dry-run.
// Output is JSON with counts, codes and digests only: no Customer names and no record ids.
func Run(ctx context.Context, pool *pgxpool.Pool, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: pms client-scope dry-run|apply --mapping FILE [--expect-digest DIGEST]")
	}
	sub := args[0]
	flags := flag.NewFlagSet("client-scope "+sub, flag.ContinueOnError)
	file := flags.String("mapping", "", "approved mapping JSON file")
	expect := flags.String("expect-digest", "", "digest printed by the reviewed dry-run")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("--mapping is required")
	}
	raw, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	m, err := ParseMapping(raw)
	if err != nil {
		return err
	}
	switch sub {
	case "dry-run":
		rep, err := DryRun(ctx, pool, m)
		if err != nil {
			return err
		}
		return print(out, rep)
	case "apply":
		if *expect == "" {
			return errors.New("apply needs --expect-digest from the reviewed dry-run")
		}
		after, err := Apply(ctx, pool, m, *expect)
		if err != nil {
			return err
		}
		return print(out, after)
	}
	return fmt.Errorf("unknown client-scope command %q", sub)
}

func print(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
