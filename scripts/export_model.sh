#!/bin/bash
# YOLO Model Export Script
# This script exports the YOLOv8 model to ONNX format for use with Go

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODEL_NAME="yolov8n.pt"
OUTPUT_NAME="yolov8n.onnx"
ASSETS_DIR="${SCRIPT_DIR}/../assets"

echo "=============================================="
echo "YOLOv8 to ONNX Export Script"
echo "=============================================="

# Check if ultralytics is installed
if ! python -c "import ultralytics" 2>/dev/null; then
    echo "Error: ultralytics is not installed."
    echo "Install with: pip install ultralytics"
    exit 1
fi

# Check if model exists
if [ ! -f "${SCRIPT_DIR}/${MODEL_NAME}" ]; then
    echo "Downloading YOLOv8 model..."
    python -c "from ultralytics import YOLO; YOLO('${MODEL_NAME}')"
fi

# Create assets directory if it doesn't exist
mkdir -p "${ASSETS_DIR}"

# Export model to ONNX
echo "Exporting ${MODEL_NAME} to ONNX format..."
python -c "
from ultralytics import YOLO
model = YOLO('${MODEL_NAME}')
model.export(format='onnx', imgsz=640, simplify=True)
import shutil
shutil.move('${OUTPUT_NAME}', '${ASSETS_DIR}/${OUTPUT_NAME}')
"

echo "=============================================="
echo "Model exported successfully!"
echo "Output: ${ASSETS_DIR}/${OUTPUT_NAME}"
echo "=============================================="

# Verify the file exists
if [ -f "${ASSETS_DIR}/${OUTPUT_NAME}" ]; then
    SIZE=$(du -h "${ASSETS_DIR}/${OUTPUT_NAME}" | cut -f1)
    echo "File size: ${SIZE}"
else
    echo "Warning: Output file not found!"
    exit 1
fi
