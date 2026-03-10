#!/usr/bin/env python3
"""
Download intfloat/multilingual-e5-small and export to ONNX format.
Also copies tokenizer files needed for Go inference.

Usage:
    pip install transformers optimum[onnxruntime] sentencepiece
    python scripts/setup_model.py
"""

import os
import sys
import shutil

def main():
    try:
        from optimum.onnxruntime import ORTModelForFeatureExtraction
        from transformers import AutoTokenizer
    except ImportError:
        print("ERROR: Required packages not installed.")
        print("Run: pip install transformers optimum[onnxruntime] sentencepiece")
        sys.exit(1)

    model_name = "intfloat/multilingual-e5-small"
    output_dir = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "models", "multilingual-e5-small")
    os.makedirs(output_dir, exist_ok=True)

    onnx_path = os.path.join(output_dir, "model.onnx")
    if os.path.exists(onnx_path):
        print(f"ONNX model already exists at {onnx_path}")
        print("Delete it if you want to re-export.")
        return

    print(f"Downloading and exporting {model_name} to ONNX...")
    print(f"Output directory: {output_dir}")

    # Export to ONNX using optimum
    model = ORTModelForFeatureExtraction.from_pretrained(model_name, export=True)
    tokenizer = AutoTokenizer.from_pretrained(model_name)

    # Save the ONNX model and tokenizer
    model.save_pretrained(output_dir)
    tokenizer.save_pretrained(output_dir)

    # Rename the onnx model file if needed
    onnx_files = [f for f in os.listdir(output_dir) if f.endswith('.onnx')]
    if onnx_files and 'model.onnx' not in onnx_files:
        src = os.path.join(output_dir, onnx_files[0])
        shutil.move(src, onnx_path)
        print(f"Renamed {onnx_files[0]} -> model.onnx")

    print(f"\n✅ Export complete!")
    print(f"Files in {output_dir}:")
    for f in sorted(os.listdir(output_dir)):
        size = os.path.getsize(os.path.join(output_dir, f))
        print(f"  {f:40s} {size:>12,} bytes")

    # Quick validation
    print("\nValidating ONNX model...")
    test_text = "query: 테스트 문장입니다."
    inputs = tokenizer(test_text, return_tensors="np", padding=True, truncation=True, max_length=512)
    outputs = model(**inputs)
    embedding = outputs.last_hidden_state[0].mean(axis=0)
    print(f"  Input: {test_text}")
    print(f"  Output shape: {embedding.shape}")
    print(f"  First 5 values: {embedding[:5]}")
    print(f"\n✅ Validation passed! Embedding dim = {embedding.shape[0]}")

if __name__ == "__main__":
    main()
