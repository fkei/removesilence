# removesilence

removesilence removes periods of silence from a video file using ffmpeg.

 Example usage:

```
go build removesilence.go

./removesilence -infile in.mp4 -outfile out.mp4  -maxpause 2 -silencedb -30
``` 
