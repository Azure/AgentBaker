// Command genregions keeps e2e/config/regions_generated.go in sync with the regions that
// E2E scenarios actually pin themselves to.
//
// The gallery replication set has to be a fixed list so that every concurrent writer submits
// the same desired state (see the comment in e2e/config/regions.go). A fixed list that drifts
// away from the scenarios is worse than no list at all, so this tool derives it mechanically
// rather than leaving it to be maintained by hand.
//
//	go run ./hack/genregions -write   # regenerate the list
//	go run ./hack/genregions -check   # fail if the list is stale (CI)
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	generatedFileName = "config/regions_generated.go"
	regionsFileName   = "config/regions.go"
	regionConstPrefix = "Region"
	baselineConstName = "BaselineRegion"
)

func main() {
	var (
		dir   = flag.String("dir", "..", "path to the e2e module root")
		write = flag.Bool("write", false, "rewrite the generated file")
		check = flag.Bool("check", false, "exit non-zero if the generated file is stale")
	)
	flag.Parse()

	root, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("resolving -dir: %v", err)
	}

	regions, err := scan(root)
	if err != nil {
		log.Fatalf("scanning scenarios: %v", err)
	}

	want := render(regions)
	target := filepath.Join(root, generatedFileName)

	switch {
	case *write:
		if err := os.WriteFile(target, want, 0o644); err != nil {
			log.Fatalf("writing %s: %v", target, err)
		}
		fmt.Printf("wrote %s\n", target)
	case *check:
		got, err := os.ReadFile(target)
		if err != nil {
			log.Fatalf("reading %s: %v", target, err)
		}
		if !bytes.Equal(got, want) {
			log.Fatalf("%s is out of date; run `make generate-e2e-regions`", generatedFileName)
		}
		fmt.Printf("%s is up to date\n", generatedFileName)
	default:
		os.Stdout.Write(want)
	}
}

// scenarioRegions holds the regions scenarios pin, split by the OS of the image they use.
// Windows is tracked separately because Windows images are much larger than Linux ones, so
// they are only replicated where a Windows scenario actually runs.
type scenarioRegions struct {
	linux   []string
	windows []string
}

func scan(root string) (scenarioRegions, error) {
	regionsPath := filepath.Join(root, regionsFileName)
	consts, err := regionConstants(regionsPath)
	if err != nil {
		return scenarioRegions{}, err
	}
	baseline, err := baselineRegion(regionsPath, consts)
	if err != nil {
		return scenarioRegions{}, err
	}

	files, err := filepath.Glob(filepath.Join(root, "*_test.go"))
	if err != nil {
		return scenarioRegions{}, err
	}
	sort.Strings(files)

	fset := token.NewFileSet()
	parsed := make([]*ast.File, 0, len(files))
	for _, path := range files {
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return scenarioRegions{}, fmt.Errorf("parsing %s: %w", path, err)
		}
		parsed = append(parsed, file)
	}

	// Regions reach a Scenario either directly or through a helper's location parameter, so
	// the argument positions of those parameters are collected first and checked at call
	// sites. Without that, runScenarioUbuntu2204GPU(t, sku, "newregion") would pin a region
	// the generated list never learns about.
	locationParams := locationParameters(parsed)

	linux := map[string]struct{}{}
	windows := map[string]struct{}{}
	var violations []string

	for _, file := range parsed {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			used, unresolved := regionsUsedIn(fn, consts, locationParams)
			target := linux
			if usesWindowsImage(fn) {
				target = windows
			}
			for _, region := range used {
				target[region] = struct{}{}
			}
			for _, expr := range unresolved {
				violations = append(violations, fmt.Sprintf("%s: %s pins an unresolvable region: %s; use a config.%s* constant so the replication set stays derivable",
					fset.Position(fn.Pos()), fn.Name.Name, expr, regionConstPrefix))
			}
		}
	}

	if len(violations) > 0 {
		return scenarioRegions{}, fmt.Errorf("scenarios pin regions the generator cannot resolve:\n  %s", strings.Join(violations, "\n  "))
	}

	// Scenarios that pin nothing run in config.BaselineRegion, so both images must be there.
	linux[baseline] = struct{}{}
	windows[baseline] = struct{}{}

	return scenarioRegions{linux: sorted(linux), windows: sorted(windows)}, nil
}

// regionsUsedIn returns the region constant values referenced anywhere in fn, plus any
// Scenario Location value the generator cannot resolve to a region constant. Regions reach a scenario both as a
// struct field (Location: config.RegionWestUS2) and as a helper argument
// (runScenarioUbuntu2204GPU(t, sku, config.RegionWestUS2)), so the whole function body is
// walked for constants rather than just Scenario composite literals.
func regionsUsedIn(fn *ast.FuncDecl, consts map[string]string, locationParams map[string][]int) (regions []string, unresolved []string) {
	params := parameterNames(fn)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			callee, ok := node.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			for _, index := range locationParams[callee.Name] {
				if index >= len(node.Args) {
					continue
				}
				if arg := node.Args[index]; !isRegionConstant(arg, consts) && !isParameter(arg, params) {
					unresolved = append(unresolved, fmt.Sprintf("%s passed as the location argument to %s", types.ExprString(arg), callee.Name))
				}
			}
		case *ast.SelectorExpr:
			pkg, ok := node.X.(*ast.Ident)
			if !ok || pkg.Name != "config" {
				return true
			}
			if value, ok := consts[node.Sel.Name]; ok {
				regions = append(regions, value)
			}
		case *ast.CompositeLit:
			// Scoped to Scenario literals so unrelated types that happen to have a Location
			// field (for example the fake request type in cache_test.go) are not flagged.
			if !isScenarioLiteral(node) {
				return true
			}
			unresolved = append(unresolved, scenarioLocationViolations(node, consts, params)...)
		}
		return true
	})
	return regions, unresolved
}

func isScenarioLiteral(lit *ast.CompositeLit) bool {
	ident, ok := lit.Type.(*ast.Ident)
	return ok && ident.Name == "Scenario"
}

// scenarioLocationViolations returns a description of every Scenario Location value that is
// not a config.Region* constant. Anything else - a raw literal, a local variable, a function
// call - cannot be resolved statically, and silently skipping it would let a scenario pin a
// region that never makes it into the replication set. That is precisely the
// GalleryImageNotFound failure this generator exists to prevent, so it is reported instead.
func scenarioLocationViolations(lit *ast.CompositeLit, consts map[string]string, params map[string]struct{}) []string {
	var found []string
	for _, element := range lit.Elts {
		kv, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "Location" {
			continue
		}
		// A parameter is fine: the value comes from a caller, and callers are scanned too.
		if isRegionConstant(kv.Value, consts) || isParameter(kv.Value, params) {
			continue
		}
		if basic, ok := kv.Value.(*ast.BasicLit); ok && basic.Kind == token.STRING {
			if value, err := strconv.Unquote(basic.Value); err == nil {
				found = append(found, fmt.Sprintf("%q", value))
				continue
			}
		}
		found = append(found, types.ExprString(kv.Value))
	}
	return found
}

// locationParameters maps each helper to the argument positions of its string parameters
// named "location", so a region passed in as an argument can be validated at the call site.
func locationParameters(files []*ast.File) map[string][]int {
	params := map[string][]int{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Type.Params == nil {
				continue
			}
			index := 0
			for _, field := range fn.Type.Params.List {
				names := field.Names
				if len(names) == 0 {
					index++
					continue
				}
				for _, name := range names {
					if name.Name == "location" {
						params[fn.Name.Name] = append(params[fn.Name.Name], index)
					}
					index++
				}
			}
		}
	}
	return params
}

func parameterNames(fn *ast.FuncDecl) map[string]struct{} {
	names := map[string]struct{}{}
	if fn.Type.Params == nil {
		return names
	}
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			names[name.Name] = struct{}{}
		}
	}
	return names
}

func isParameter(expr ast.Expr, params map[string]struct{}) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	_, ok = params[ident.Name]
	return ok
}

func isRegionConstant(expr ast.Expr, consts map[string]string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "config" {
		return false
	}
	_, ok = consts[selector.Sel.Name]
	return ok
}

// usesWindowsImage reports whether fn builds a scenario around a Windows VHD. The VHD is
// always named through a config.VHDWindows* identifier, so the name is enough and there is no
// need to resolve types.
func usesWindowsImage(fn *ast.FuncDecl) bool {
	windows := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		selector, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if strings.HasPrefix(selector.Sel.Name, "VHDWindows") {
			windows = true
			return false
		}
		return true
	})
	return windows
}

// regionConstants reads the hand-maintained Region* constants so the scan compares against
// exactly the names declared in regions.go, not a second copy of them.
func regionConstants(path string) (map[string]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	consts := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != len(value.Values) {
				continue
			}
			for i, name := range value.Names {
				if !strings.HasPrefix(name.Name, regionConstPrefix) {
					continue
				}
				literal, ok := value.Values[i].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					continue
				}
				unquoted, err := strconv.Unquote(literal.Value)
				if err != nil {
					continue
				}
				consts[name.Name] = unquoted
			}
		}
	}
	if len(consts) == 0 {
		return nil, fmt.Errorf("no %s* constants found in %s", regionConstPrefix, path)
	}
	return consts, nil
}

// baselineRegion resolves config.BaselineRegion, which every image is replicated to whether
// or not a scenario names it. It is read from regions.go rather than duplicated here so the
// generated file cannot disagree with the constant the runtime actually uses.
func baselineRegion(path string, consts map[string]string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", path, err)
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != len(value.Values) {
				continue
			}
			for i, name := range value.Names {
				if name.Name != baselineConstName {
					continue
				}
				switch expr := value.Values[i].(type) {
				case *ast.Ident:
					if region, ok := consts[expr.Name]; ok {
						return region, nil
					}
					return "", fmt.Errorf("%s in %s refers to unknown constant %s", baselineConstName, path, expr.Name)
				case *ast.BasicLit:
					if expr.Kind == token.STRING {
						return strconv.Unquote(expr.Value)
					}
				}
				return "", fmt.Errorf("%s in %s must be a region constant or string literal, got %s", baselineConstName, path, types.ExprString(value.Values[i]))
			}
		}
	}
	return "", fmt.Errorf("no %s constant found in %s", baselineConstName, path)
}

func sorted(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func render(regions scenarioRegions) []byte {
	var buf bytes.Buffer
	buf.WriteString("// Code generated by hack/genregions; DO NOT EDIT.\n")
	buf.WriteString("// Regenerate with `make generate-e2e-regions` after changing a scenario's region.\n\n")
	buf.WriteString("package config\n\n")

	buf.WriteString("// scenarioRegions is every region a non-Windows E2E scenario pins itself to. Shared\n")
	buf.WriteString("// gallery image versions are replicated to all of them regardless of which region the\n")
	buf.WriteString("// current test needs, so concurrent writers agree on the target region list.\n")
	buf.WriteString("// config.BaselineRegion is always included.\n")
	writeSlice(&buf, "scenarioRegions", regions.linux)

	buf.WriteString("\n// windowsScenarioRegions is every region a Windows E2E scenario pins itself to. It is\n")
	buf.WriteString("// tracked separately because Windows images are large, so they are not replicated to\n")
	buf.WriteString("// regions only Linux scenarios use. config.BaselineRegion is always included.\n")
	writeSlice(&buf, "windowsScenarioRegions", regions.windows)

	return buf.Bytes()
}

func writeSlice(buf *bytes.Buffer, name string, values []string) {
	if len(values) == 0 {
		fmt.Fprintf(buf, "var %s = []string{}\n", name)
		return
	}
	fmt.Fprintf(buf, "var %s = []string{\n", name)
	for _, value := range values {
		fmt.Fprintf(buf, "\t%q,\n", value)
	}
	buf.WriteString("}\n")
}
