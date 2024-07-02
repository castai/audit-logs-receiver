#!/bin/bash

# Define the repository URL
REPO_URL="https://github.com/open-telemetry/opentelemetry-collector"

# Define the directory name
DIR_NAME="temp"

# Create the directory
mkdir -p $DIR_NAME

# Clone the repository into the temporary directory
git clone $REPO_URL $DIR_NAME

# Change directory to the cmd/mdatagen directory
cd $DIR_NAME/cmd/mdatagen || { echo "Failed to cd into mdatagen directory"; exit 1; }

# Run go install
go install

# Run mdatagen with the specified metadata.yaml file
mdatagen ../../../auditlogsreceiver/metadata.yaml
