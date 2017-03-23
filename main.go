package main

// Integration giddyup as a go generate tool means that each generate will increment the next
// dev version. For a different workflow, go generate might not be a good fit but you can still
// use giddyup independently
//go:generate giddyup

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"gopkg.in/alecthomas/kingpin.v2"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	app            = kingpin.New("giddyup", "A go generate tool to increment an application's version.")
	variable       = kingpin.Flag("variable", "Name of the version variable.").Default("VERSION").String()
	mode           = kingpin.Flag("mode", "Increment mode (MAJOR, MINOR, PATCH)").Short('m').Default("PATCH").Enum("MAJOR", "MINOR", "PATCH")
	paths          = kingpin.Arg("paths", "directories or files").Strings()
	currentVersion = kingpin.Flag("toolVersion", "Only prints the current version of the tool without incrementing for the next release").Short('t').Default("false").Bool()
	verbose        = kingpin.Flag("verbose", "Verbose output (prints current and next dev versions)").Short('v').Default("false").Bool()
	lazyInit       = kingpin.Flag("init", "Initialize version to 1.0.0 if no managed version found)").Short('i').Default("false").Bool()
)

type errWriter struct {
	b   *bytes.Buffer
	err error
}

func (ew *errWriter) writeString(value string) {
	if ew.err != nil {
		return
	}
	_, ew.err = ew.b.WriteString(value)
}

func main() {
	kingpin.Version(VERSION)
	kingpin.Parse()

	inputPaths := *paths
	if len(inputPaths) == 0 {
		// Default: process whole package in current directory.
		inputPaths = []string{"."}
	}

	if *currentVersion {
		if err := printCurrentVersion(*variable, inputPaths); err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current version: %v", err)
		}
	} else {
		if err := run(*variable, inputPaths, *mode, *lazyInit); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating version: %v", err)
		}
	}
}

func printCurrentVersion(variable string, inputPaths []string) error {
	for _, path := range inputPaths {
		version, err := getCurrentVersion(path, false)
		if err != nil {
			return err
		}
		fmt.Printf("%s", version)
	}

	return nil
}

func run(variable string, inputPaths []string, mode string, lazyInit bool) error {
	for _, path := range inputPaths {
		version, err := getCurrentVersion(path, lazyInit)
		if err != nil {
			return err
		}

		if *verbose {
			fmt.Printf("Current version is [%s]\n", version)
		}

		nextDevVersion, err := getNextVersion(version, mode)
		if err != nil {
			return err
		}

		if *verbose {
			fmt.Printf("Next dev version is [%s]\n", nextDevVersion)
		}

		var buffer bytes.Buffer
		if err := writeHeader(&buffer, variable); err != nil {
			return err
		}
		pkg := parsePackage(path)

		err = generateContent(pkg, nextDevVersion, variable, &buffer)
		if err != nil {
			return err
		}

		// Write to file.
		output := fmt.Sprintf("%s/version.go", filepath.Dir(path))

		err = ioutil.WriteFile(output, buffer.Bytes(), 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

// isFile reports whether the named file is a file (not a directory).
func isFile(name string) bool {
	info, err := os.Stat(name)
	if err != nil {
		log.Fatal(err)
	}
	return !info.IsDir()
}

func getCurrentVersion(path string, lazyInit bool) (string, error) {
	fset := token.NewFileSet()

	versionFilePath := filepath.Join(path, "version.go")
	f, err := parser.ParseFile(fset, versionFilePath, nil, 0)
	if err != nil {
		if lazyInit {
			if *verbose {
				fmt.Printf("Version file not found at [%s], initializing version to [1.0.0]\n", versionFilePath)
			}

			return "1.0.0", nil
		}
		return "", err
	}

	for _, decl := range f.Decls {
		switch decl := decl.(type) {
		case *ast.GenDecl:
			if decl.Tok == token.CONST {
				for _, spec := range decl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valueSpec.Names {
							if name.String() == *variable {
								for _, value := range valueSpec.Values {
									if basicLiteral, ok := value.(*ast.BasicLit); ok {
										return strings.Trim(basicLiteral.Value, "\""), nil
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("Could not find version constant [%s] in file [%s]", *variable, versionFilePath)
}

func getNextVersion(version string, mode string) (string, error) {
	versionRegEx := regexp.MustCompile("\\A(\\d)+\\.(\\d)\\.(\\d)+\\z")

	if versionRegEx.MatchString(version) {
		matches := versionRegEx.FindAllStringSubmatch(version, -1)[0]
		majorVersion, err := strconv.Atoi(matches[1])
		if err != nil {
			return "", fmt.Errorf("Version format should be [number.number.number] but was [%s]: [%v]", version, err)
		}

		minorVersion, err := strconv.Atoi(matches[2])
		if err != nil {
			return "", fmt.Errorf("Version format should be [number.number.number] but was [%s]: [%v]", version, err)
		}

		patchVersion, err := strconv.Atoi(matches[3])
		if err != nil {
			return "", fmt.Errorf("Version format should be [number.number.number] but was [%s]: [%v]", version, err)
		}

		switch mode {
		case "PATCH":
			patchVersion = patchVersion + 1
		case "MINOR":
			minorVersion = minorVersion + 1
		case "MAJOR":
			majorVersion = majorVersion + 1
		}

		return fmt.Sprintf("%d.%d.%d", majorVersion, minorVersion, patchVersion), nil
	} else {
		return "", fmt.Errorf("Version format should be [number.number.number] but was [%s]", version)
	}
}

// writeHeader writes the header of the file (code generation warning as well as the go:generate line)
func writeHeader(buffer *bytes.Buffer, variable string) error {
	ew := &errWriter{b: buffer}
	ew.writeString(fmt.Sprintln("// GENERATED and MANAGED by giddyup (https://github.com/alexandre-normand/giddyup)"))

	return ew.err
}

func generateContent(pkg string, version string, variable string, buffer *bytes.Buffer) error {
	buffer.WriteString(fmt.Sprintf("package %s\n\nconst (\n\t%s = \"%s\"\n)\n", pkg, variable, version))

	return nil
}

// parsePackage analyzes the single package constructed from the named files.
// If text is non-nil, it is a string to be used instead of the content of the file,
// to be used for testing. parsePackage exits if there is an error.
func parsePackage(directory string) string {
	var astFiles []*ast.File
	fs := token.NewFileSet()
	files, err := ioutil.ReadDir(directory)
	if err != nil {
		log.Fatalf("Failed to read directory [%s]: %v", directory, err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".go") {
			continue
		}
		parsedFile, err := parser.ParseFile(fs, file.Name(), nil, 0)
		if err != nil {
			log.Fatalf("parsing package: %s: %s", file.Name(), err)
		}
		astFiles = append(astFiles, parsedFile)
	}
	if len(astFiles) == 0 {
		log.Fatalf("%s: no buildable Go files", directory)
	}
	return astFiles[0].Name.Name
}
