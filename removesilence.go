package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type options struct {
	infile    string
	outfile   string
	silencedb float64
}

type period struct {
	start float64
	end   float64
}

func main() {
	opts, err := getOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stdout, "options: %v\n", opts)

	// survey
	p, err := survey(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "survey error.\n")
	}

	// cut
	if err := cut(opts, p); err != nil {
		fmt.Fprintf(os.Stderr, "cut error.\n")
	}

	// end
	fmt.Fprintf(os.Stdout, "output %s\n", opts.outfile)
}

func survey(opts *options) (*period, error) {
	cmd := exec.Command("time", "ffmpeg",
		// "-v", "trace",
		"-i", opts.infile,
		"-filter_complex", fmt.Sprintf("[0:a]silencedetect=noise=%gdB:duration=%g[outa]", opts.silencedb, 0.1),
		"-map", "[outa]",
		"-f", "s16le",
		"-y", "/dev/null",
	)

	fmt.Fprintf(os.Stdout, "command(survey): %s\n", cmd.Args)

	lines, err := getCmdResults(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, strings.Join(lines, "\n")+"\n")
		return nil, err
	}
	duration, _ := getDuration(lines)
	fmt.Fprintf(os.Stdout, "duration %v\n", duration)

	p, err := getPeriod(lines)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stdout, "period %v\n", p)

	return p, nil
}

func cut(opts *options, p *period) error {

	cmd := exec.Command("time", "ffmpeg",
		// "-v", "trace",
		"-ss", strconv.FormatFloat(p.start, 'f', 2, 64),
		"-i", opts.infile,
		"-t", strconv.FormatFloat(p.end-p.start, 'f', 2, 64),
		"-vcodec", "copy", // speed up
		"-acodec", "copy", // speed up
		"-y",
		opts.outfile,
	)
	fmt.Fprintf(os.Stdout, "command(cut): %s\n", cmd.Args)

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func getPeriod(lines []string) (*period, error) {

	p := &period{
		start: 0.0,
		end:   0.0,
	}
	// fmt.Fprintf(os.Stdout, "lines %v\n", lines)

	for i := len(lines) - 1; i >= 0; i-- {
		words := strings.Split(lines[i], " ")
		if len(words) < 5 {
			continue
		}
		if words[3] == "silence_start:" {
			// fmt.Fprintf(os.Stdout, "silence_start %v\n", words[4])
			v, err := strconv.ParseFloat(words[4], 64)
			if err != nil {
				return nil, err
			}
			// fmt.Fprintf(os.Stdout, "end %v\n", v)
			p.end = v
			break
		}
	}

	for _, line := range lines {
		words := strings.Split(line, " ")
		if len(words) < 5 {
			continue
		}
		if words[3] == "silence_start:" {
			// fmt.Fprintf(os.Stdout, "silence_start %v\n", words[4])
			v, err := strconv.ParseFloat(words[4], 64)
			if err != nil {
				return nil, err
			}
			// fmt.Fprintf(os.Stdout, "end %v\n", v)
			p.start = v
			break
		}
	}

	return p, nil
}

func getDuration(lines []string) (float64, error) {
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
	return 0.0, errors.New("couldn't find duration in ffmpeg output")
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

func getCmdResults(cmd *exec.Cmd) ([]string, error) {
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

func getOptions() (*options, error) {
	infile := flag.String("infile", "", "Path to input video file.")
	outfile := flag.String("outfile", "", "Path to output video file.")
	silencedb := flag.Float64("silencedb", -30.0, "V volume level (dB) below which audio is considered to be silence.	Usually negative (e.g. -30).")

	flag.Parse()

	// check
	if *infile == "" {
		return nil, errors.New("command-line options `-infile` is required")
	}
	if *outfile == "" {
		return nil, errors.New("command-line options `-outfile` is required")
	}
	if *silencedb == 0.0 {
		return nil, errors.New("command-line options `-silencedb` is required")
	}

	return &options{
		infile:    *infile,
		outfile:   *outfile,
		silencedb: *silencedb,
	}, nil
}
