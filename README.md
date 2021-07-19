# removesilence

// forked from jordancurve/removesilence

Sample code that cuts the front and back silence in the video using `ffmpeg`.

> ja. 動画中の前後の無音部分をカットするサンプルコード

Dependence

- Commend : `ffmpeg`

Usage of removesilence:

```bash
Required flags:
  -infile string
      Path to input video file.
  -outfile string
      Path to output video file.
  -silencedb float
      volume level (dB) below which audio is considered to be silence.
      Usually negative (e.g. -30).
```

Example:

```bash
go build removesilence.go

./removesilence -infile in.mp4 -outfile out.mp4 -silencedb -30
```
