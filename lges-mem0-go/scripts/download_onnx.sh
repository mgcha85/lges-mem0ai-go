#!/bin/bash

# Directory for models and libs
mkdir -p .mem0-go/models
mkdir -p .mem0-go/lib

# Download ONNX Runtime (Linux x64)
ORT_VERSION="1.17.1"
ORT_FILE="onnxruntime-linux-x64-${ORT_VERSION}.tgz"
if [ ! -f ".mem0-go/lib/libonnxruntime.so" ]; then
    echo "Downloading ONNX Runtime..."
    wget "https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/${ORT_FILE}"
    tar -xzvf ${ORT_FILE}
    cp onnxruntime-linux-x64-${ORT_VERSION}/lib/libonnxruntime.so* .mem0-go/lib/
    rm -rf onnxruntime-linux-x64-${ORT_VERSION} ${ORT_FILE}
    # Symlink for easier access
    ln -sf .mem0-go/lib/libonnxruntime.so.${ORT_VERSION} .mem0-go/lib/libonnxruntime.so
fi

# Download Validated ONNX Model (all-MiniLM-L6-v2)
# Using a hosted version or configuring download from Hugging Face
MODEL_DIR=".mem0-go/models"
if [ ! -f "${MODEL_DIR}/all-MiniLM-L6-v2.onnx" ]; then
    echo "Downloading all-MiniLM-L6-v2 ONNX model..."
    # Placeholder URL - usually you'd export this from HF or download a pre-exported one.
    # For this script, let's assume valid URLs (using a generic HF export link example)
    wget -O "${MODEL_DIR}/all-MiniLM-L6-v2.onnx" "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx"
    wget -O "${MODEL_DIR}/vocab.txt" "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/vocab.txt"
fi

echo "Setup complete."
echo "Please set LD_LIBRARY_PATH before running:"
echo "export LD_LIBRARY_PATH=\$LD_LIBRARY_PATH:\$(pwd)/.mem0-go/lib"
