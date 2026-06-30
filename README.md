# demografix CLI

Command-line client for the Demografix APIs — [genderize.io](https://genderize.io),
[agify.io](https://agify.io), and [nationalize.io](https://nationalize.io). One
static binary covering all three services, plus spreadsheet enrichment that
joins predictions back onto your rows.

## Install

One line (Linux, macOS):

```sh
curl -fsSL https://raw.githubusercontent.com/DemografixGenderize/demografix-cli/main/install.sh | sh
```

Homebrew:

```sh
brew install demografixgenderize/tap/demografix
```

Or download a binary for your OS/arch from the [releases](https://github.com/DemografixGenderize/demografix-cli/releases).

## Authentication

Every request requires an API key. The key is resolved in this order:

1. `--api-key-file <path>`
2. `DEMOGRAFIX_API_KEY` environment variable
3. the config file written by `demografix login`

There is no raw `--api-key` flag (it would leak into shell history). `login`
stores the key at `~/.config/demografix/config.toml` with `0600` permissions.

```sh
demografix login            # prompts for the key, no echo, verifies it
```

## Usage

```sh
# Predictions take names as arguments or one per line on stdin.
demografix gender peter lois meg
demografix gender andrea --country IT
printf 'peter\nlois\n' | demografix gender
demografix age peter
demografix nationality nguyen

# Output is a table on a terminal and JSONL when piped; override with -o.
demografix gender peter -o json
demografix nationality nguyen -o jsonl

# Remaining quota for your key.
demografix quota
```

### Spreadsheet enrichment

`enrich` reads a CSV/TSV/JSONL/JSON/XLSX file, predicts for each row, and writes
the original columns plus the prediction columns — preserving every other
column. It mirrors the browser tool's output.

```sh
demografix enrich people.csv -o out.csv \
  --gender --age --nationality \
  --name-col full_name \          # or --first-name-col / --last-name-col
  --country-col country \         # or --country US (gender/age only)
  --top-n 3 \                     # nationality candidate columns
  --prefix pred_ \                # avoid output-column collisions
  --resume \                      # only fill rows missing predictions
  --dry-run                       # validate and print cost, make no calls
```

## Output formats

`-o table|json|jsonl|tsv|csv`. The default is `table` on a terminal and `jsonl`
when the output is piped.

## Exit codes

`0` ok · `2` usage · `3` auth · `4` subscription · `5` validation · `6` rate
limit · `7` transport.
