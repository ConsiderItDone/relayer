#!/bin/bash

# Define the directory and file
DATA_DIR="./data"
BC_DATA_DIR="$DATA_DIR/ibc-1/data"
TARGET_FILE="priv_validator_state.json"

# Remove all files and folders except the target file
find "$BC_DATA_DIR" -mindepth 1 ! -name "$TARGET_FILE" -delete

# Update the height field in the target file using sed
sed -i '' 's/"height": "[0-9]*"/"height": "0"/' "$BC_DATA_DIR/$TARGET_FILE"

# remove ibc-1.log from the data directory
rm -f "$DATA_DIR/ibc-1.log"
