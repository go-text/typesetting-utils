# Code generator for Unicode properties, in Golang 

This script generates lookup function for Unicode properties not
covered by the standard package `unicode`.

The properties are meant to be consumed by text processing libraries, such as segmenters or shapers.

## Usage 

The `go run main.go <OUTPUT_DIR>` command will generate files in three directories (which must exists):

    - <OUTPUT_DIR>/harfbuzz
    - <OUTPUT_DIR>/unicodedata
    - <OUTPUT_DIR>/language


