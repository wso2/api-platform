echo "Starting the devportal-webapp..."

if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    chmod +x ./devportal-webapp-macos
    exec ./devportal-webapp-macos
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux
    chmod +x ./devportal-webapp-linux
    exec ./devportal-webapp-linux
else
    echo "Unsupported operating system: $OSTYPE"
    exit 1
fi
