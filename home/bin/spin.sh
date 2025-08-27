#!/usr/bin/env bash

declare -a spin=(
▎ '|'
▎ '{'
▎ '<'
  '-'
  ' '
▎ '-'
▎ '>'
▎ '}'
)

while :; do
▎ for frame in "${spin[@]}"; do
▎ ▎ echo -ne "\r$frame"
▎ ▎ sleep 0.1  # Adjust speed as needed
▎ done
done
echo
