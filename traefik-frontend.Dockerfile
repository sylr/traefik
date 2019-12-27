# Base image
FROM node:12.11

# Create working dir
ENV WEBUI_DIR /src/webui
RUN mkdir -p $WEBUI_DIR
WORKDIR $WEBUI_DIR

# Copy dependency manifests first so that updates to the sources do not invalidate
# the docker cache of the next step which download all dependencies
COPY webui/package.json webui/package-lock.json ./

# Download dependencies
RUN npm install

# Copy sources
COPY webui/ ./

# Build frontend
RUN npm run lint
RUN npm run build:nc
