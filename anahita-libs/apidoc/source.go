package apidoc

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// What the source says of a package's types: the comment above each type,
// and the comment on each of its fields. A field's comment is the
// description it is given, so nothing is worded twice.

type typeComments struct {
	doc    string
	fields map[string]string
}

type sources struct {
	// From where packages are looked up: a directory in the module that
	// uses them, so that its replace directives are followed.
	dir      string
	packages map[string]map[string]*typeComments
}

func newSources(dir string) *sources {
	return &sources{dir: dir, packages: map[string]map[string]*typeComments{}}
}

// comments is what the source says of one type, or nil for a type with no
// source (one of Go's own) or none found.
func (s *sources) comments(pkgPath, typeName string) (*typeComments, error) {
	if pkgPath == "" {
		return nil, nil
	}
	// A generic type is named with its arguments: "List[...]".
	if open := strings.Index(typeName, "["); open >= 0 {
		typeName = typeName[:open]
	}
	types, ok := s.packages[pkgPath]
	if !ok {
		var err error
		types, err = s.read(pkgPath)
		if err != nil {
			return nil, err
		}
		s.packages[pkgPath] = types
	}
	return types[typeName], nil
}

func (s *sources) read(pkgPath string) (map[string]*typeComments, error) {
	types := map[string]*typeComments{}
	// Go's own packages are not described from their source.
	if !strings.Contains(strings.Split(pkgPath, "/")[0], ".") && !s.isOurs(pkgPath) {
		return types, nil
	}
	pkg, err := build.Import(pkgPath, s.dir, build.FindOnly)
	if err != nil {
		return nil, fmt.Errorf("finding the source of %s: %w", pkgPath, err)
	}
	entries, err := os.ReadDir(pkg.Dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(pkg.Dir, name), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts := spec.(*ast.TypeSpec)
				tc := &typeComments{fields: map[string]string{}}
				switch {
				case ts.Doc != nil:
					tc.doc = firstParagraph(ts.Doc.Text())
				case len(gen.Specs) == 1 && gen.Doc != nil:
					tc.doc = firstParagraph(gen.Doc.Text())
				}
				if st, ok := ts.Type.(*ast.StructType); ok {
					for _, field := range st.Fields.List {
						text := ""
						if field.Doc != nil {
							text = field.Doc.Text()
						} else if field.Comment != nil {
							text = field.Comment.Text()
						}
						for _, name := range field.Names {
							tc.fields[name.Name] = firstParagraph(text)
						}
					}
				}
				types[ts.Name.Name] = tc
			}
		}
	}
	return types, nil
}

// isOurs says whether a package is one of this repository's: a module's
// path here has no dot in its first part ("anahita-libs/...", "hashtag/...").
// Go's own packages have none either, and are told apart by being in GOROOT.
func (s *sources) isOurs(pkgPath string) bool {
	pkg, err := build.Import(pkgPath, s.dir, build.FindOnly)
	return err == nil && !pkg.Goroot
}

// firstParagraph is a comment as a description: its first paragraph, its
// lines run together. As in Go's own documentation, the first paragraph says
// what a thing is; what follows it is for whoever works on the code (why a
// field is never left out, what broke before), and stays out of the
// reference.
func firstParagraph(text string) string {
	paragraph, _, _ := strings.Cut(strings.TrimSpace(text), "\n\n")
	return strings.Join(strings.Fields(paragraph), " ")
}
