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

var keepTempFiles bool

func main() {
	inFile := flag.String("infile", "", "Path to input video file.")
	outFile := flag.String("outfile", "", "Path to output video file.")
	silenceDb := flag.Float64("silencedb", 0.0,
		"volume level (dB) below which audio is considered to be silence. "+
			"Usually negative (e.g. -30).")
	introPadding := flag.Float64("intropadding", 0.0,
		"number of seconds of video to keep before you start talking for the first time. ")
	outroPadding := flag.Float64("outropadding", 0.0,
		"number of seconds of video to keep after you stop talking for the last time. ")
	minKeep := flag.Float64("minkeep", 0.0, "minimum length of segment to include (including any padding).")
	maxPause := flag.Float64("maxpause", 0.0, "max allowable period of silence "+
		"aside from intro/outro (in seconds). Any silent segment longer than this "+
		"will be trimmed down to exactly this length by removing the middle "+
		"portion and leaving maxpause/2 seconds of padding on each side. After this is done, "+
		"any remaining clips shorter than maxpause+1 seconds will be further shortened.")
	flag.BoolVar(&keepTempFiles, "keep-temp-files", false, "keep temp files")

	flag.Parse()
	if err := doit(*inFile, *outFile, *maxPause, *minKeep, *silenceDb, *introPadding, *outroPadding); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func doit(inFile, outFile string, maxPause, minKeep, silenceDb float64, introPadding, outroPadding float64) error {
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
	if !keepTempFiles {
		defer os.RemoveAll(tmpDir)
	}
	cmd := exec.Command("nice", "ffmpeg",
		"-i", inFile,
		"-filter_complex", fmt.Sprintf("[0:a]silencedetect=n=%gdB:d=%g[outa]", silenceDb, 0.1),
		"-map", "[outa]",
		"-f", "s16le",
		"-y", "/dev/null",
	)
	lines, err := commandStderrLines(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, strings.Join(lines, "\n")+"\n")
		return err
	}
	dur, err := ffmpegParseDuration(lines)
	if err != nil {
		return err
	}
	silence, err := ffmpegParseSilentPeriods(lines)
	if err != nil {
		return err
	}
	keep := segmentsToKeep(silence, maxPause, introPadding, outroPadding, dur)
	if minKeep > 0.0 {
		keep = stripSegmentsShorterThan(keep, minKeep)
	}
	shortenPaddingForShortClips(keep, maxPause)
	fmt.Printf("keeping segments: %v\n", keep)
	chunks, err := cut(inFile, keep, tmpDir)
	if err != nil {
		return err
	}
	return join(chunks, outFile, tmpDir)
}

func shortenPaddingForShortClips(keep []segment, maxPause float64) {
	p := maxPause / 2 // padding
	for i, c := range keep {
		if i == 0 || i == len(keep)-1 {
			continue
		}
		if c.duration() < 2*p+1 {
			newPad := c.duration() - 2*p
			padDiff := p - newPad
			if newPad < p {
				keep[i].start += padDiff
				keep[i].end -= padDiff
				fmt.Printf("shortened padding: %s --> %s\n", c, keep[i])
			}
		}
	}
}

func stripSegmentsShorterThan(segs []segment, thresh float64) []segment {
	keep := []segment{}
	for _, s := range segs {
		if s.duration() >= thresh {
			keep = append(keep, s)
		}
	}
	return keep
}

func (s segment) duration() float64 {
	return s.end - s.start
}

func segmentsToKeep(silence []segment, maxPause, introPadding, outroPadding, dur float64) []segment {
	return invertSegments(segmentsToRemove(silence, maxPause, introPadding, outroPadding, dur))
}

func ffmpegParseDuration(lines []string) (float64, error) {
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] != "Duration:" {
			continue
		}
		d := fields[1]
		s, err := parseHMS(strings.TrimSuffix(d, ","))
		if err != nil {
			return 0, err
		}
		return s, nil
	}
	return 0.0, errors.New("Couldn't find duration in ffmpeg output")
}

func parseHMS(hms string) (float64, error) {
	x := strings.Split(hms, ":")
	h, err := strconv.ParseFloat(x[0], 64)
	if err != nil {
		return 0, err
	}
	m, err := strconv.ParseFloat(x[1], 64)
	if err != nil {
		return 0, err
	}
	s, err := strconv.ParseFloat(x[2], 64)
	if err != nil {
		return 0, err
	}
	return 60*60*h + 60*m + s, nil
}

func join(inFiles []string, outFile, tmpDir string) error {
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
		"nice",
		"ffmpeg",
		"-f", "concat",
		"-safe", "0",
		"-i", fileList,
		"-c", "copy",
		"-map", "0",
		"-y",
		outFile,
	)
	cmd.Stderr = logFile
	cmd.Stdout = logFile
	fmt.Printf("%s\n", strings.Join(cmd.Args, " "))
	return cmd.Run()
}

func cut(inFile string, keep []segment, tmpDir string) ([]string, error) {
	chunks := []string{}
	logFilePath := filepath.Join(tmpDir, "extract.log")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		return nil, err
	}
	ext := filepath.Ext(inFile)
	for i, k := range keep {
		args := []string{"ffmpeg", "-v", "error"}
		if k.start != 0 {
			args = append(args, "-ss", fmt.Sprintf("%f", k.start))
		}
		args = append(args, "-i", inFile)
		if k.end != 0 {
			args = append(args, "-t", fmt.Sprintf("%f", k.end-k.start))
		}
		args = append(args, "-acodec", "copy")
		chunk := filepath.Join(tmpDir, fmt.Sprintf("%d%s", i+1, ext))
		chunks = append(chunks, chunk)
		args = append(args, chunk)
		cmd := exec.Command("nice", args...)
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		fmt.Printf("%s\n", strings.Join(cmd.Args, " "))
		if err := cmd.Run(); err != nil {
			showFile(logFilePath)
			return nil, err
		}
	}
	return chunks, nil
}

func segmentsToRemove(silence []segment, maxPause, introPadding, outroPadding, dur float64) []segment {
	rem := []segment{}
	if len(silence) == 0 {
		return []segment{}
	}
	if introPadding > 0 && silence[0].start <= 0 && silence[0].end > introPadding {
		rem = append(rem, segment{0, silence[0].end - introPadding})
	}
	for _, s := range silence[1:] {
		if s.duration() > maxPause && s.end < dur {
			rem = append(rem, segment{s.start + maxPause/2.0, s.end - maxPause/2.0})
		}
	}
	e := silence[len(silence)-1]
	if outroPadding > 0 && e.end >= dur && e.duration() > outroPadding {
		rem = append(rem, segment{e.start + outroPadding, 0.0})
	}
	return rem
}

func invertSegments(segs []segment) []segment {
	end := 0.0
	inv := []segment{}
	fmt.Printf("inverting %v\n", segs)
	for _, s := range segs {
		if s.start > 0.0 {
			inv = append(inv, segment{end, s.start})
		}
		end = s.end
	}
	inv = append(inv, segment{end, 0.0})
	return inv
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
	o := fmt.Sprintf("%.2f-", s.start)
	if s.end != 0.0 {
		o += fmt.Sprintf("%.2f", s.end)
	}
	return o
}

func showFile(path string) {
	contents, err := ioutil.ReadFile(path)
	if err == nil {
		fmt.Print(string(contents))
	}
}
