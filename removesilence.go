// removesilence removes periods of silence from a video file using ffmpeg.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type segment struct {
	start, end float64
}

var debug bool

func main() {
	inFile := flag.String("infile", "", "Path to input video file.")
	outFile := flag.String("outfile", "", "Path to output video file.")
	silenceDb := flag.Float64("silencedb", 0.0,
		"volume level (dB) below which audio is considered to be silence. "+
			"Usually negative (e.g. -30).")
	maxPause := flag.Float64("maxpause", 0.0, "max allowable period of silence (seconds). "+
		"Any silent segment longer than this will be trimmed down to exactly this "+
		"length by removing the middle portion and leaving maxpause/2 seconds of "+
		"padding on each side.")
	flag.BoolVar(&debug, "debug", false, "debug mode (preserve temp directory)")

	flag.Parse()
	if err := doit(*inFile, *outFile, *maxPause, *silenceDb); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func doit(inFile, outFile string, maxPause, silenceDb float64) error {
	if inFile == "" {
		return errors.New("-infile is required")
	}
	if outFile == "" {
		return errors.New("-outfile is required")
	}
	if maxPause == 0.0 {
		return errors.New("-maxpause is required")
	}
	if silenceDb == 0.0 {
		return errors.New("-silencedb is required")
	}

	tmpDir, err := ioutil.TempDir("/tmp", "rs")
	if err != nil {
		return err
	}
	if !debug {
		defer os.RemoveAll(tmpDir)
	}
	cmd := exec.Command("ffmpeg",
		"-i", inFile,
		"-filter_complex", fmt.Sprintf("[0:a]silencedetect=n=%gdB:d=%g[outa]", silenceDb, maxPause),
		"-map", "[outa]",
		"-f", "s16le",
		"-y", "/dev/null",
	)
	lines, err := commandStderrLines(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, strings.Join(lines, "\n")+"\n")
		return err
	}
	silence, err := ffmpegParseSilentPeriods(lines)
	if err != nil {
		return err
	}
	fmt.Printf("silent segments: %v\n", silence)
	keep := invertSegmentsWithPadding(silence, maxPause/2.0)
	fmt.Printf("keeping segments: %v\n", keep)
	chunks, err := ffmpegExtractSegments(inFile, keep, tmpDir)
	if err != nil {
		return err
	}
	return ffmpegConcatenateChunks(chunks, outFile, tmpDir)
}

func ffmpegConcatenateChunks(inFiles []string, outFile, tmpDir string) error {
	logFile, err := os.Create(filepath.Join(tmpDir, "concat.log"))
	if err != nil {
		return err
	}
	lines := []string{}
	for _, f := range inFiles {
		lines = append(lines, fmt.Sprintf("file '%s'\n", f))
	}
	fileList := filepath.Join(tmpDir, "list.txt")
	if err := ioutil.WriteFile(fileList, []byte(strings.Join(lines, "")), 0600); err != nil {
		return err
	}
	cmd := exec.Command(
		"ffmpeg",
		"-f", "concat",
		"-safe", "0", // https://www.ffmpeg.org/ffmpeg-formats.html#Options
		"-i", fileList,
		"-c", "copy",
		"-map", "0",
		"-y",
		outFile,
	)
	cmd.Stderr = logFile
	cmd.Stdout = logFile
	fmt.Printf("%s %s\n", cmd.Path, strings.Join(cmd.Args, " "))
	return cmd.Run()
}

func ffmpegExtractSegments(inFile string, keep []segment, tmpDir string) ([]string, error) {
	chunks := []string{}
	logFile, err := os.Create(filepath.Join(tmpDir, "extract.log"))
	if err != nil {
		return nil, err
	}
	ext := filepath.Ext(inFile)
	// https://superuser.com/a/863451/99065
	for i, k := range keep {
		args := []string{
			"-i", inFile,
			"-y",
			"-avoid_negative_ts", "1",
			"-copyinkf",
			"-acodec", "copy",
			"-vcodec", "copy",
			"-scodec", "copy",
		}
		chunk := filepath.Join(tmpDir, fmt.Sprintf("%d%s", i, ext))
		chunks = append(chunks, chunk)
		if k.start != 0 {
			args = append(args, "-ss", fmt.Sprintf("%f", k.start))
		}
		if k.end != 0 {
			args = append(args, "-t", fmt.Sprintf("%f", k.end-k.start))
		}
		args = append(args,
			"-map_metadata", "0",
			"-avoid_negative_ts", "1",
		)
		args = append(args, chunk)
		cmd := exec.Command("ffmpeg", args...)
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		fmt.Printf("%s %s\n", cmd.Path, strings.Join(cmd.Args, " "))
		if err := cmd.Run(); err != nil {
			return nil, err
		}
	}
	return chunks, nil
}

func invertSegmentsWithPadding(silence []segment, padding float64) []segment {
	end := padding
	keep := []segment{}
	for _, s := range silence {
		keep = append(keep, segment{end - padding, s.start + padding})
		end = s.end
	}
	keep = append(keep, segment{end - padding, 0.0})
	return keep
}

func ffmpegParseSilentPeriods(lines []string) ([]segment, error) {
	silence := []segment{}
	for _, line := range lines {
		words := strings.Split(line, " ")
		if len(words) < 5 {
			continue
		}
		tag, val := words[3], words[4]
		loc, err := strconv.ParseFloat(val, 64)
		if err != nil {
			continue
		}
		switch tag {
		case "silence_start:":
			silence = append(silence, segment{loc, 0.0})
		case "silence_end:":
			if len(silence) == 0 {
				return nil, errors.New("ffmpeg output: silence_end before silence_start")
			}
			silence[len(silence)-1].end = loc
		}
	}
	return silence, nil
}

func commandStderrLines(cmd *exec.Cmd) ([]string, error) {
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(stderr)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return lines, err
	}
	if err := cmd.Wait(); err != nil {
		return lines, err
	}
	return lines, nil
}

func (s segment) String() string {
	o := fmt.Sprintf("%g-", s.start)
	if s.end != 0.0 {
		o += fmt.Sprintf("%g", s.end)
	}
	return o
}
