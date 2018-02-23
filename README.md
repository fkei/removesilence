# removesilence

removesilence: remove silent segments from a video file using ffmpeg.

Usage of removesilence:
```
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
  -debug
      debug mode (preserve temp directory)
```

Example:

```
go build removesilence.go

./removesilence -infile in.mp4 -outfile out.mp4  -maxpause 2 -silencedb -30
``` 
