# removesilence

removesilence: remove silent segments from a video file using ffmpeg.

Usage of removesilence:
```
Required flags:
  -infile string
      Path to input video file.
  -outfile string
      Path to output video file.
  -maxpause float
      max allowable period of silence (seconds). Any silent segment longer than
      this will be trimmed down to exactly this length by removing the middle
      portion and leaving maxpause/2 seconds of padding on each side.
  -silencedb float
      volume level (dB) below which audio is considered to be silence.
      Usually negative (e.g. -30).

Optional flags:

  -minkeep float
    	minimum length of segment to include (including any padding).
  -keep-temp-files
      keep temp files
  -intropadding float
    	number of seconds of video to keep before you start talking for the first time.
  -outropadding float
    	number of seconds of video to keep after you stop talking for the last time.
```

Example:

```
go build removesilence.go

./removesilence -infile in.mp4 -outfile out.mp4  -maxpause 2 -silencedb -30 -minkeep 2.4
``` 
