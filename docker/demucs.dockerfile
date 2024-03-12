# Use the same base image as the original
FROM xserrat/facebook-demucs:latest

# Set the working directory
WORKDIR /usr/lib/demucs

# Copy entrypoint script into the container
COPY demucs_entrypoint.sh /demucs_entrypoint.sh

# Ensure the entrypoint script is executable
RUN chmod +x /demucs_entrypoint.sh

# Set the entrypoint script to run when the container starts
ENTRYPOINT ["/demucs_entrypoint.sh"]