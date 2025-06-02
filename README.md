# TedFetch

A CLI tool to download TED talk videos and subtitles.

## Installation

```sh
go install github.com/baiyutang/tedfetch@latest
```

## Usage

### Download a TED talk by title

```sh
tedfetch download "The power of vulnerability" --quality 720p
```

### Download a TED talk by URL

```sh
tedfetch download https://www.ted.com/talks/ariel_ekblaw_how_to_build_in_space_for_life_on_earth --quality 720p --subtitle zh-CN
```

### Command Options

- `--quality, -q`: Video quality (720p, 1080p). Default: 720p.
- `--subtitle, -s`: Subtitle language code (e.g., en, zh-CN). Leave empty to skip subtitle download.
- `--output, -o`: Output directory. Default: current directory.

## Development

### Build

```sh
make build
```

### Run Tests

```sh
make test
```

### Lint

```sh
make lint
```

### Clean

```sh
make clean
```
