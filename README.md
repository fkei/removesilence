# removesilence

removesilence: remove silent segments from a video file using ffmpeg.

```
Usage of ./removesilence:
  -debug
    	debug mode (preserve temp directory)
  -infile string
    	Path to input video file.
  -maxpause float
    	max allowable period of silence. Anything longer than this amount will be trimmed down to this amount by removing an equal period of silence from both edges.
  -outfile string
    	Path to output video file.
  -silencedb float
    	dB value under which audio is considered to be silence.
```

Example:

```
go build removesilence.go

./removesilence -infile in.mp4 -outfile out.mp4  -maxpause 2 -silencedb -30
``` 
