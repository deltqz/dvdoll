package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

const (
	colorRed    = "\x1b[91m"
	colorYellow = "\x1b[93m"
	colorCyan   = "\x1b[96m"
	colorGray   = "\x1b[90m"
	colorReset  = "\x1b[0m"
)

type options struct {
	inputPath      string
	title          int
	chapterStart   string
	chapterEnd     int
	startTime      string
	endTime        string
	outputFile     string
	listOnly       bool
	nonInteractive bool
}

func main() {
	opts := parseFlags()

	fmt.Printf("%sDVDoll - FFmpeg DVD remuxer%s\n", colorRed, colorReset)
	fmt.Println()
	if err := ensureExecutable("ffprobe"); err != nil {
		fatal(err.Error())
	}
	if err := ensureExecutable("ffmpeg"); err != nil {
		fatal(err.Error())
	}

	for {
		if opts.inputPath == "" {
			opts.inputPath = prompt("Input", opts.nonInteractive)
		}
		if opts.inputPath == "" {
			fatal("Input path is required.")
		}
		opts.inputPath = strings.Trim(opts.inputPath, "\"")
		opts.inputPath = filepath.Clean(opts.inputPath)

		printTitleList(opts.inputPath)
		if opts.listOnly {
			return
		}

		if opts.title == 0 {
			titleStr := prompt("Title number", opts.nonInteractive)
			if titleStr == "" {
				fatal("Title number is required.")
			}
			fmt.Sscanf(titleStr, "%d", &opts.title)
		}
		if opts.title <= 0 {
			fatal("Title number must be greater than zero.")
		}

		if opts.chapterStart == "" {
			opts.chapterStart = prompt("First chapter", opts.nonInteractive)
			if opts.chapterStart == "" {
				fatal("First chapter is required.")
			}
		}

		useTime := opts.chapterStart == "0" || strings.Contains(opts.chapterStart, ":")
		chapterStartInt := 0
		if useTime {
			if opts.startTime == "" {
				if opts.chapterStart == "0" {
					opts.startTime = prompt("Start time (-ss)", opts.nonInteractive)
				} else {
					opts.startTime = opts.chapterStart
				}
			}
			opts.startTime = normalizeStartTime(opts.startTime)
			if opts.endTime == "" {
				opts.endTime = prompt("End time (-to)", opts.nonInteractive)
			}
			opts.endTime = normalizeEndTime(opts.endTime)
		} else {
			fmt.Sscanf(opts.chapterStart, "%d", &chapterStartInt)
			if chapterStartInt <= 0 {
				fatal("First chapter must be greater than zero.")
			}
			if opts.chapterEnd == 0 {
				chapterEndStr := prompt("Last chapter", opts.nonInteractive)
				if chapterEndStr == "" {
					fatal("Last chapter is required.")
				}
				fmt.Sscanf(chapterEndStr, "%d", &opts.chapterEnd)
			}
			if opts.chapterEnd <= 0 {
				fatal("Last chapter must be greater than zero.")
			}
		}

		if opts.outputFile == "" {
			opts.outputFile = prompt("Output filename", opts.nonInteractive)
		}
		if opts.outputFile == "" {
			opts.outputFile = "output"
		}
		if !strings.HasSuffix(strings.ToLower(opts.outputFile), ".mkv") {
			opts.outputFile += ".mkv"
		}

		audioCodec, err := detectAudioCodec(opts.inputPath, opts.title)
		if err != nil {
			fatal(err.Error())
		}

		audioArgs := []string{"-c:a", "copy"}
		if strings.Contains(strings.ToLower(audioCodec), "pcm") {
			audioArgs = []string{"-c:a", "flac", "-compression_level:a", "8"}
		}

		args := []string{"-hide_banner", "-f", "dvdvideo", "-preindex", "True", "-title", fmt.Sprintf("%d", opts.title)}
		if useTime {
			args = append(args, "-ss", opts.startTime)
			if opts.endTime != "" {
				args = append(args, "-to", opts.endTime)
			}
		} else {
			args = append(args, "-chapter_start", fmt.Sprintf("%d", chapterStartInt), "-chapter_end", fmt.Sprintf("%d", opts.chapterEnd))
		}
		args = append(args, "-i", opts.inputPath, "-map", "0", "-c", "copy")
		args = append(args, audioArgs...)
		args = append(args, opts.outputFile)

		fmt.Println()
		fmt.Println("Remuxing file...")
		if err := runCommand("ffmpeg", args...); err != nil {
			fatal(err.Error())
		}
		fmt.Println("Process finished.")

		if opts.nonInteractive {
			return
		}
		if !promptContinue(false) {
			return
		}

		clearScreen()
		printHeader()

		opts.inputPath = ""
		opts.title = 0
		opts.chapterStart = ""
		opts.chapterEnd = 0
		opts.startTime = ""
		opts.endTime = ""
		opts.outputFile = ""
		opts.listOnly = false
	}
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.inputPath, "input", "", "Input DVD path or ISO file")
	flag.IntVar(&opts.title, "title", 0, "Title number")
	flag.StringVar(&opts.chapterStart, "chapter-start", "", "First chapter or time (0 or HH:MM:SS)")
	flag.IntVar(&opts.chapterEnd, "chapter-end", 0, "Last chapter")
	flag.StringVar(&opts.startTime, "start-time", "", "Start time (-ss) in HH:MM:SS")
	flag.StringVar(&opts.endTime, "end-time", "", "End time (-to) in HH:MM:SS")
	flag.StringVar(&opts.outputFile, "output", "", "Output filename (mkv)")
	flag.BoolVar(&opts.listOnly, "list", false, "Only list titles and exit")
	flag.BoolVar(&opts.nonInteractive, "non-interactive", false, "Disable prompts and fail on missing values")
	flag.Parse()

	if opts.inputPath == "" && flag.NArg() > 0 {
		opts.inputPath = flag.Arg(0)
	}
	return opts
}

func printTitleList(inputPath string) {
	fmt.Println()
	fmt.Printf("%sTitle list:%s\n", colorYellow, colorReset)
	found := false

	for title := 1; title <= 99; title++ {
		duration, err := getTitleDuration(inputPath, title)
		if err != nil || duration == "" {
			break
		}
		if strings.EqualFold(duration, "N/A") {
			duration = "00:00:00"
		}

		chapters, _ := getChapterCount(inputPath, title)
		fmt.Printf("  Title %d - Chapters: %02d (%s)\n", title, chapters, duration)
		found = true
	}

	if !found {
		fmt.Println("  No titles found.")
	}
	fmt.Println()
}

func getTitleDuration(inputPath string, title int) (string, error) {
	args := []string{
		"-v", "error",
		"-hide_banner",
		"-f", "dvdvideo",
		"-preindex", "True",
		"-title", fmt.Sprintf("%d", title),
		"-show_entries", "format=duration",
		"-sexagesimal",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputPath,
	}
	out, err := runCommandCapture("ffprobe", args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func getChapterCount(inputPath string, title int) (int, error) {
	args := []string{
		"-v", "error",
		"-hide_banner",
		"-f", "dvdvideo",
		"-preindex", "True",
		"-title", fmt.Sprintf("%d", title),
		"-show_chapters",
		"-show_entries", "chapter=index",
		"-of", "csv=p=0",
		inputPath,
	}
	out, err := runCommandCapture("ffprobe", args...)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
		return 0, nil
	}
	return len(lines), nil
}

func detectAudioCodec(inputPath string, title int) (string, error) {
	fmt.Println()
	fmt.Println("Analyzing audio codec...")
	args := []string{
		"-v", "error",
		"-hide_banner",
		"-f", "dvdvideo",
		"-preindex", "True",
		"-title", fmt.Sprintf("%d", title),
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputPath,
	}
	out, err := runCommandCapture("ffprobe", args...)
	if err != nil {
		return "", err
	}
	codec := strings.TrimSpace(out)
	if codec == "" {
		return "", errors.New("no audio codec detected. Check if ffprobe is in your PATH and the input is valid")
	}
	fmt.Printf("Detected audio codec: %s\n", codec)
	return codec, nil
}

func normalizeStartTime(input string) string {
	in := strings.TrimSpace(strings.ToLower(input))
	if in == "" || in == "0" || in == "first" || in == "start" {
		return "00:00:00"
	}
	return input
}

func normalizeEndTime(input string) string {
	in := strings.TrimSpace(strings.ToLower(input))
	if in == "" || in == "last" || in == "end" {
		return ""
	}
	return input
}

func prompt(label string, nonInteractive bool) string {
	if nonInteractive {
		return ""
	}
	fmt.Printf("%s%s: %s", colorCyan, label, colorReset)
	reader := bufio.NewReader(os.Stdin)
	value, _ := reader.ReadString('\n')
	return strings.TrimSpace(value)
}

func promptContinue(nonInteractive bool) bool {
	if nonInteractive {
		return false
	}
	fmt.Printf("%sPress %sEnter%s to process another file%s\n", colorGray, colorCyan, colorGray, colorReset)
	fmt.Printf("%sPress %sESC%s to close%s\n", colorGray, colorRed, colorGray, colorReset)

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		s := prompt("(press Enter to continue, type 'esc' to exit)", false)
		if strings.ToLower(strings.TrimSpace(s)) == "esc" {
			return false
		}
		return true
	}
	defer term.Restore(fd, oldState)

	buf := make([]byte, 1)
	_, err = os.Stdin.Read(buf)
	if err != nil {
		return false
	}
	b := buf[0]
	if b == 27 {
		return false
	}
	if b == '\r' || b == '\n' {
		return true
	}
	return true
}

func clearScreen() {
	fmt.Print("\x1b[2J\x1b[H")
}

func printHeader() {
	fmt.Printf("%sDVDoll - FFmpeg DVD remuxer%s\n", colorRed, colorReset)
	fmt.Println()
}

func ensureExecutable(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%s not found in PATH", name)
	}
	return nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runCommandCapture(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func fatal(message string) {
	fmt.Printf("%sERROR:%s %s\n", colorRed, colorReset, message)
	os.Exit(1)
}
