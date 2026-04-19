// lspq is a standalone CLI for querying language servers via the lsp package.
//
// Usage: lspq [--json] <command> [flags] <file> [line] [symbol-or-column]
//
//	def      Go to definition
//	refs     Find all references
//	hover    Get type info and docs
//	rename   Rename symbol (requires -to flag)
//	symbols  List all symbols in a file
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/dotcommander/piglet-extensions/lsp"
)

// ---------------------------------------------------------------------------
// JSON output shapes
// ---------------------------------------------------------------------------

type jsonLocation struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type jsonDefOutput struct {
	Definition *jsonLocation `json:"definition"`
}

type jsonRefsOutput struct {
	References []jsonLocation `json:"references"`
}

type jsonHoverOutput struct {
	Hover string `json:"hover"`
}

type jsonSymbol struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type jsonSymbolsOutput struct {
	Symbols []jsonSymbol `json:"symbols"`
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	// Support both `lspq --json refs ...` and `lspq refs --json ...`.
	// Parse a root-level flag set first, then hand off the remainder.
	root := flag.NewFlagSet("lspq", flag.ContinueOnError)
	jsonFlag := root.Bool("json", false, "Output JSON instead of human-readable text")
	root.SetOutput(os.Stderr)
	// Ignore errors — we want unrecognised flags to fall through to the
	// sub-command flag set so that `-to` and `-col` are still accepted.
	_ = root.Parse(os.Args[1:])
	remaining := root.Args()

	if len(remaining) < 1 {
		usage()
		os.Exit(1)
	}

	cmd := remaining[0]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	to := fs.String("to", "", "New name for rename command")
	col := fs.Int("col", 0, "Column (1-based); auto-detected if symbol name given")
	// Allow --json after the subcommand too.
	jsonSub := fs.Bool("json", false, "Output JSON instead of human-readable text")

	if err := fs.Parse(remaining[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	args := fs.Args()

	jsonMode := *jsonFlag || *jsonSub

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cwd, err := os.Getwd()
	if err != nil {
		fatalf("getwd: %v", err)
	}

	if err := run(ctx, cmd, args, *to, *col, cwd, jsonMode); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd string, args []string, to string, colFlag int, cwd string, jsonMode bool) error {
	mgr := lsp.NewManager(cwd)
	defer mgr.Shutdown(context.Background())

	switch cmd {
	case "symbols":
		return cmdSymbols(ctx, mgr, args, cwd, jsonMode)
	case "def", "refs", "hover", "rename":
		return cmdPosition(ctx, mgr, cmd, args, to, colFlag, cwd, jsonMode)
	default:
		usage()
		return fmt.Errorf("unknown command: %q", cmd)
	}
}

func cmdSymbols(ctx context.Context, mgr *lsp.Manager, args []string, cwd string, jsonMode bool) error {
	if len(args) < 1 {
		return fmt.Errorf("symbols requires <file>")
	}
	file := resolveFile(args[0], cwd)

	client, lang, err := mgr.ForFile(ctx, file)
	if err != nil {
		return err
	}
	if err := mgr.EnsureFileOpen(ctx, client, file, lang); err != nil {
		return err
	}
	syms, err := client.DocumentSymbols(ctx, file)
	if err != nil {
		return fmt.Errorf("symbols: %w", err)
	}

	if jsonMode {
		return writeJSON(buildSymbolsJSON(syms, file))
	}
	fmt.Println(lsp.FormatSymbols(syms, cwd))
	return nil
}

func cmdPosition(ctx context.Context, mgr *lsp.Manager, cmd string, args []string, to string, colFlag int, cwd string, jsonMode bool) error {
	if len(args) < 2 {
		return fmt.Errorf("%s requires <file> <line> [symbol]", cmd)
	}

	file := resolveFile(args[0], cwd)

	lineNum, err := strconv.Atoi(args[1])
	if err != nil || lineNum < 1 {
		return fmt.Errorf("line must be a positive integer, got %q", args[1])
	}
	line := lineNum - 1 // convert to 0-based

	col, err := resolveCol(file, line, args[2:], colFlag)
	if err != nil {
		return err
	}

	client, lang, err := mgr.ForFile(ctx, file)
	if err != nil {
		return err
	}
	if err := mgr.EnsureFileOpen(ctx, client, file, lang); err != nil {
		return err
	}

	switch cmd {
	case "def":
		locs, err := client.Definition(ctx, file, line, col)
		if err != nil {
			return fmt.Errorf("definition: %w", err)
		}
		if jsonMode {
			return writeJSON(buildDefJSON(locs, cwd))
		}
		fmt.Println(lsp.FormatLocations(locs, cwd, 2))

	case "refs":
		locs, err := client.References(ctx, file, line, col)
		if err != nil {
			return fmt.Errorf("references: %w", err)
		}
		if jsonMode {
			return writeJSON(buildRefsJSON(locs, cwd))
		}
		fmt.Println(lsp.FormatLocations(locs, cwd, 1))

	case "hover":
		hover, err := client.Hover(ctx, file, line, col)
		if err != nil {
			return fmt.Errorf("hover: %w", err)
		}
		if jsonMode {
			return writeJSON(buildHoverJSON(hover))
		}
		fmt.Println(lsp.FormatHover(hover))

	case "rename":
		if to == "" {
			return fmt.Errorf("rename requires -to <new-name>")
		}
		edit, err := client.Rename(ctx, file, line, col, to)
		if err != nil {
			return fmt.Errorf("rename: %w", err)
		}
		fmt.Println(lsp.FormatWorkspaceEdit(edit, cwd))
	}

	return nil
}

// ---------------------------------------------------------------------------
// JSON builders
// ---------------------------------------------------------------------------

func buildDefJSON(locs []lsp.Location, cwd string) jsonDefOutput {
	if len(locs) == 0 {
		return jsonDefOutput{Definition: nil}
	}
	loc := locs[0]
	return jsonDefOutput{Definition: &jsonLocation{
		File:   resolveURIToRel(loc.URI, cwd),
		Line:   loc.Range.Start.Line + 1,
		Column: loc.Range.Start.Character + 1,
	}}
}

func buildRefsJSON(locs []lsp.Location, cwd string) jsonRefsOutput {
	out := jsonRefsOutput{References: make([]jsonLocation, 0, len(locs))}
	for _, loc := range locs {
		out.References = append(out.References, jsonLocation{
			File:   resolveURIToRel(loc.URI, cwd),
			Line:   loc.Range.Start.Line + 1,
			Column: loc.Range.Start.Character + 1,
		})
	}
	return out
}

func buildHoverJSON(hover *lsp.HoverResult) jsonHoverOutput {
	if hover == nil {
		return jsonHoverOutput{}
	}
	return jsonHoverOutput{Hover: hover.Contents.Value}
}

func buildSymbolsJSON(syms []lsp.DocumentSymbol, file string) jsonSymbolsOutput {
	out := jsonSymbolsOutput{Symbols: make([]jsonSymbol, 0, len(syms))}
	flattenSymbols(&out.Symbols, syms, file)
	return out
}

func flattenSymbols(dst *[]jsonSymbol, syms []lsp.DocumentSymbol, file string) {
	for _, s := range syms {
		*dst = append(*dst, jsonSymbol{
			Name:   s.Name,
			Kind:   s.Kind.String(),
			File:   file,
			Line:   s.Range.Start.Line + 1,
			Column: s.Range.Start.Character + 1,
		})
		if len(s.Children) > 0 {
			flattenSymbols(dst, s.Children, file)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeJSON(v any) error {
	return json.NewEncoder(os.Stdout).Encode(v)
}

// resolveURIToRel converts an LSP URI to a relative path from cwd.
func resolveURIToRel(uri, cwd string) string {
	path := uriToPath(uri)
	if cwd == "" {
		return path
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil {
		return path
	}
	return rel
}

// uriToPath converts a file:// URI to a filesystem path, decoding percent-encoded characters.
func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return uri
	}
	return u.Path
}

// resolveCol determines the 0-based column from remaining positional args or the -col flag.
func resolveCol(file string, line int, rest []string, colFlag int) (int, error) {
	if len(rest) > 0 {
		sym := rest[0]
		if _, err := strconv.Atoi(sym); err != nil {
			// Non-numeric: treat as symbol name, auto-detect column.
			return lsp.FindSymbolColumn(file, line, sym)
		}
	}
	if colFlag > 0 {
		return colFlag - 1, nil // convert 1-based to 0-based
	}
	return 0, nil
}

func resolveFile(file, cwd string) string {
	if filepath.IsAbs(file) {
		return file
	}
	return filepath.Join(cwd, file)
}

func usage() {
	fmt.Fprint(os.Stderr, `lspq - LSP query tool

Usage:
  lspq [--json] <command> [flags] <file> [line] [symbol]

Commands:
  def      Go to definition
  refs     Find all references
  hover    Get type info and docs
  rename   Rename symbol (requires -to flag)
  symbols  List all symbols in a file

Flags:
  --json       Output JSON instead of human-readable text
  -to string   New name for rename command
  -col int     Column (1-based); auto-detected if symbol name given

JSON output shapes:
  def:     {"definition": {"file": "...", "line": N, "column": N}}
  refs:    {"references": [{"file": "...", "line": N, "column": N}, ...]}
  hover:   {"hover": "..."}
  symbols: {"symbols": [{"name": "...", "kind": "...", "file": "...", "line": N, "column": N}, ...]}

Examples:
  lspq def main.go 42 HandleRequest
  lspq --json refs server.go 10 -col 5
  lspq hover auth.go 55 Validate
  lspq rename auth.go 55 Validate -to ValidateToken
  lspq symbols main.go
  lspq --json symbols main.go
`)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "lspq: "+format+"\n", args...)
	os.Exit(1)
}
