package main

import "bytes"
import "fmt"
import "os"
import "os/exec"
import "io/ioutil"
import "io"
import "strings"
import "golang.org/x/image/draw"
import "image/jpeg"
import "image/png"
import "image"
import "path"
import "path/filepath"

var inkscapeBin = "inkscape"
var force = false

func isOlder(src string, than string) bool {
	if force {
		return false
	}
	stat, err := os.Stat(src)

	// If the file does not exist, it is not older.
	if os.IsNotExist(err) {
		return false
	}

	statThan, _ := os.Stat(than)

	return stat.ModTime().After(statThan.ModTime())
}

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func rescale(ssrc string, sdst string) {

	input, _ := os.Open(ssrc)
	defer input.Close()

	output, err := os.Create(sdst)
	defer output.Close()
	if err != nil {
		panic(fmt.Sprintf("Error creating %s", sdst))
	}

	// Decode the image (from PNG to image.Image):
	var src image.Image
	if strings.Contains(ssrc, ".png") {
		src, err = png.Decode(input)
	} else {
		src, err = jpeg.Decode(input)
	}

	if err != nil {
		panic(fmt.Sprintf("Error loading %s", ssrc))

	}

	// Set the expected size that you want:
	dst := image.NewRGBA(image.Rect(0, 0, 100, 100.0*src.Bounds().Max.Y/src.Bounds().Max.X))

	// Resize:
	draw.NearestNeighbor.Scale(dst, dst.Rect, src, src.Bounds(), draw.Over, nil)

	// Encode to `output`:
	png.Encode(output, dst)
}

func prepareImg(src string, dst string) {
	// Compare last modified and skip if necessary
	if isOlder(dst, src) {
		return
	}

	if mode == "release" {
		// Copy
		fmt.Println("Copy:", src, "->", dst)
		copy(src, dst)
	} else {
		fmt.Println("Scale:", src, "->", dst)
		rescale(src, dst)
	}
}

func prepareDrawing(src string, dst string) {
	dst = strings.ReplaceAll(dst, ".svg", ".png")

	// Compare last modified and skip if necessary
	if isOlder(dst, src) {
		return
	}

	bin := inkscapeBin
	arg0 := "--export-filename=" + dst
	var arg1 = "--export-dpi=300"
	if mode == "debug" {
		arg1 = "--export-dpi=100"
	}
	arg2 := src

	fmt.Println(inkscapeBin, arg0, arg1, arg2)

	out, err := exec.Command(bin, arg0, arg1, arg2).CombinedOutput()

	if err != nil {
		fmt.Println("%s %s", string(out), err)
		return
	}
}

func prepare(folder string, f func(string, string)) {
	os.MkdirAll(cwd()+out+"/"+folder, os.ModePerm)
	items, _ := ioutil.ReadDir(folder)
	for _, item := range items {
		if item.IsDir() {
			os.MkdirAll(cwd()+out+"/"+folder+item.Name()+"/", os.ModePerm)
			prepare(folder+item.Name()+"/", f)
		}
		var src = cwd() + folder + item.Name()
		var dst = cwd() + out + "/" + folder + item.Name()
		f(src, dst)
	}
}

func toString(fs []string) string {
	var b bytes.Buffer
	for _, s := range fs {
		fmt.Fprintf(&b, "%s ", s)
	}
	return b.String()
}

func makeCover(src string, dst string) {
	src = cwd() + src
	dst = cwd() + dst

	if isOlder(dst, src) {
		return
	}
	bin := inkscapeBin
	args := make([]string, 0)
	args = append(args, "--export-filename="+dst)

	if mode == "debug" {
		args = append(args, "--export-dpi=100")
	} else {
		args = append(args, "--export-dpi=300")
	}
	args = append(args, "--export-type=pdf")
	args = append(args, "--export-text-to-path")
	// arg3 := "--verb"
	// arg4 := "ObjectToPath"
	// arg3 := ""
	args = append(args, src)

	fmt.Printf("%s %s\n", inkscapeBin, toString(args))

	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		fmt.Println("%s %s", string(out), err)
		return
	}
}

var mode = "release"
var out = ""

func cwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	cwd += "/"
	return cwd
}

func currentDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	return path.Base(cwd)
}

func checkExecutable(bin string) {
	path, err := exec.LookPath(bin)
	if err != nil {
		fmt.Println(fmt.Sprintf("Could not find executable '%s'", bin))
		os.Exit(1)
	}
	fmt.Println(fmt.Sprintf("Found '%s' -> '%s'", bin, path))
}

func getMode() string {
  var args = os.Args
  if len(args) > 1 {
		return args[1]
  }
  return mode
}

func preprocessLatexForMarkdown(content string) string {
	// Replace inline CJK arrow with simple arrow
	content = strings.ReplaceAll(content, `\begin{CJK}{UTF8}{min}→\end{CJK}`, `→`)
	
	// Handle drawing macros first (they need .png appended)
	// \draw{file} -> \includegraphics{illu/d/file.png}
	content = replaceDrawMacros(content, `\draw{`, `illu/d/`)
	content = replaceDrawMacros(content, `\nbdraw{`, `illu/d/`)
	
	// Convert regular image macros to standard markdown-friendly format
	// \img{file.png} -> \includegraphics{illu/img/file.png}
	content = strings.ReplaceAll(content, `\img{`, `\includegraphics{illu/img/`)
	
	// \nbimg{file.png} -> \includegraphics{illu/img/file.png}
	content = strings.ReplaceAll(content, `\nbimg{`, `\includegraphics{illu/img/`)
	
	// Handle scaled images: \sbimg{scale}{file} and \sbdraw{scale}{file}
	// These are more complex as they have two arguments
	// For now, we'll handle them with a basic approach
	// Convert \sbimg{0.5}{file.png} to \includegraphics[width=0.5\textwidth]{illu/img/file.png}
	content = replaceScaledImages(content, `\sbimg{`, `illu/img/`, false)
	content = replaceScaledImages(content, `\sbdraw{`, `illu/d/`, true)
	content = replaceScaledImages(content, `\simg{`, `illu/img/`, false)
	content = replaceScaledImages(content, `\sdraw{`, `illu/d/`, true)
	
	// Replace $\times$ with the actual × character for better markdown rendering
	content = strings.ReplaceAll(content, `$\times$`, `×`)
	
	// Replace other common math symbols with Unicode equivalents
	content = strings.ReplaceAll(content, `$\mu$`, `μ`)
	
	return content
}

func replaceDrawMacros(content string, macro string, basePath string) string {
	// This function handles macros like \draw{file} and adds .png extension
	for {
		idx := strings.Index(content, macro)
		if idx == -1 {
			break
		}
		
		// Find the argument (filename)
		start := idx + len(macro)
		braceCount := 1
		end := start
		for end < len(content) && braceCount > 0 {
			if content[end] == '{' {
				braceCount++
			} else if content[end] == '}' {
				braceCount--
			}
			end++
		}
		if braceCount != 0 {
			break // malformed
		}
		filename := content[start:end-1]
		
		// Add .png if not already present
		if !strings.HasSuffix(filename, ".png") {
			filename = filename + ".png"
		}
		
		// Build replacement
		replacement := `\includegraphics{` + basePath + filename + `}`
		
		// Replace in content
		content = content[:idx] + replacement + content[end:]
	}
	
	return content
}

func replaceScaledImages(content string, macro string, basePath string, addPng bool) string {
	// This function handles macros like \sbimg{scale}{file}
	// Convert to \includegraphics[width=scale\textwidth]{basePath/file}
	
	for {
		idx := strings.Index(content, macro)
		if idx == -1 {
			break
		}
		
		// Find the first argument (scale)
		start := idx + len(macro)
		braceCount := 1
		scaleEnd := start
		for scaleEnd < len(content) && braceCount > 0 {
			if content[scaleEnd] == '{' {
				braceCount++
			} else if content[scaleEnd] == '}' {
				braceCount--
			}
			scaleEnd++
		}
		if braceCount != 0 {
			break // malformed
		}
		scale := content[start:scaleEnd-1]
		
		// Find the second argument (filename)
		if scaleEnd >= len(content) || content[scaleEnd] != '{' {
			break // malformed
		}
		fileStart := scaleEnd + 1
		braceCount = 1
		fileEnd := fileStart
		for fileEnd < len(content) && braceCount > 0 {
			if content[fileEnd] == '{' {
				braceCount++
			} else if content[fileEnd] == '}' {
				braceCount--
			}
			fileEnd++
		}
		if braceCount != 0 {
			break // malformed
		}
		filename := content[fileStart:fileEnd-1]
		
		// Add .png if requested and not already present
		if addPng && !strings.HasSuffix(filename, ".png") {
			filename = filename + ".png"
		}
		
		// Build replacement
		replacement := `\includegraphics[width=` + scale + `\textwidth]{` + basePath + filename + `}`
		
		// Replace in content
		content = content[:idx] + replacement + content[fileEnd:]
	}
	
	return content
}

func generateMarkdown() {
	fmt.Println("Generating markdown files for each chapter...")
	
	checkExecutable("pandoc")
	
	outputDirName := "out/markdown"
	if err := os.MkdirAll(outputDirName, os.ModePerm); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		return
	}
	
	// List of chapter files to convert
	chapters := []string{
		"forewords",
		"aknowledgments",
		"bug_reports",
		"cheat_sheet",
		"introduction",
		"hardware",
		"programing",
		"gfx",
		"prog_z80",
		"prog_68000",
		"people",
		"epilogue",
		"appendix",
		"bib",
	}
	
	successCount := 0
	errorCount := 0
	
	for _, chapter := range chapters {
		srcFile := filepath.Join(cwd(), "src", chapter+".tex")
		dstFile := filepath.Join(cwd(), outputDirName, chapter+".md")
		
		// Check if source file exists
		if _, err := os.Stat(srcFile); os.IsNotExist(err) {
			fmt.Printf("Skipping %s (file not found)\n", srcFile)
			continue
		}
		
		fmt.Printf("Converting %s -> %s\n", srcFile, dstFile)
		
		// Read the file
		content, err := os.ReadFile(srcFile)
		if err != nil {
			fmt.Printf("  Error reading %s: %v\n", chapter, err)
			errorCount++
			continue
		}
		
		// Preprocess LaTeX content to handle custom macros and formatting
		processed := preprocessLatexForMarkdown(string(content))
		
		// Write to temporary file
		tmpFile := filepath.Join(cwd(), outputDirName, chapter+".tmp.tex")
		if err := os.WriteFile(tmpFile, []byte(processed), 0644); err != nil {
			fmt.Printf("  Error writing temp file: %v\n", err)
			errorCount++
			continue
		}
		
		// Use pandoc to convert LaTeX to Markdown
		bin := "pandoc"
		args := []string{
			"-f", "latex",
			"-t", "markdown",
			"--wrap=preserve",
			"-o", dstFile,
			tmpFile,
		}
		
		out, err := exec.Command(bin, args...).CombinedOutput()
		
		// Clean up temporary file
		os.Remove(tmpFile)
		
		if err != nil {
			fmt.Printf("  Warning: Error converting %s: %v\n", chapter, err)
			errorCount++
			// Try to save what was converted
			if len(out) > 0 {
				fmt.Printf("  Partial output may have been saved\n")
			}
		} else {
			successCount++
		}
	}
	
	fmt.Printf("\n✓ Markdown generation complete!\n")
	fmt.Printf("  Successfully converted: %d files\n", successCount)
	if errorCount > 0 {
		fmt.Printf("  Warnings/Errors: %d files\n", errorCount)
	}
	fmt.Printf("  Output directory: %s/\n", outputDirName)
}

func main() {
	mode = getMode()
	fmt.Println("Building in", mode, "mode...")

	// Handle markdown mode early (doesn't need inkscape)
	if mode == "markdown" {
		generateMarkdown()
		return
	}

	checkExecutable(inkscapeBin)
	
	
  var args = os.Args
	if len(args) > 2 {
		force = true
	}

	if mode != "debug" && mode != "release" && mode != "print" && mode != "markdown" {
		fmt.Println("Mode must be either 'debug', 'release', 'print', or 'markdown'.")
		return
	}

	compileOptions := ""
	if mode == "print" {
		compileOptions = `\def\forprint{}`
		mode = "release"
	}

	outputDirName := "out"
	out = outputDirName + "/" + mode
	os.MkdirAll(out, os.ModePerm)

	makeCover("src/cover/pdf/cover_front.svg", out+"/illu/cover_front.pdf")
	makeCover("src/cover/pdf/cover_back.svg", out+"/illu/cover_back.pdf")

	prepare("illu/img/", prepareImg)
	prepare("illu/d/", prepareDrawing)

	bin := "pdflatex"
	arg0 := "-output-directory"
	arg1 := outputDirName
	arg2 := `\def\base{` + out + `} ` + compileOptions + ` \input{src/book.tex}`
	
        var err error
	var out []byte
	draftMode := "-draftmode"

	// Compile in draft mode to generate only necessary files for Table of Contents
	if mode != "debug" {
		fmt.Println(bin, draftMode, arg0, arg1, arg2)
		out, err = exec.Command(bin, draftMode, arg0, arg1, arg2).CombinedOutput()
	}

	// Full Compile to generate PDF
	if err == nil {
		fmt.Println(bin, arg0, arg1, arg2)
		out, err = exec.Command(bin, arg0, arg1, arg2).CombinedOutput()
	}

	if err != nil {
		fmt.Println("%s %s", string(out), err)
        }

  // Rename
  var src = outputDirName + "/book.pdf"
  var dst =  outputDirName + "/" + currentDir() + "_" + getMode() + ".pdf"
	os.Rename(src, dst)

}
