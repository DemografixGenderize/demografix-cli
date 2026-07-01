# demografix CLI

`demografix` tells you the likely **gender**, **age**, and **nationality** of a
person from their name, right from your terminal. Predict a single name, pipe in
a list of them, or enrich a whole spreadsheet in place.

It is a single static binary, built on the [genderize.io](https://genderize.io),
[agify.io](https://agify.io), and [nationalize.io](https://nationalize.io) APIs.

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

Every request needs an API key. Create a free account at
[genderize.io](https://genderize.io) to get one — the free tier includes 2,500
names per month, and the same key works across all three services.

Log in once and it is remembered:

```sh
demografix login            # prompts for your key (no echo), verifies it, saves it
```

The key is stored at `~/.config/demografix/config.toml`, readable only by you.
For scripts and CI, set `DEMOGRAFIX_API_KEY` instead — it takes precedence over
the saved key.

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

`enrich` reads a CSV/TSV/JSONL/JSON/XLSX file, predicts for each row, and appends
the prediction columns while preserving every other column.

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
