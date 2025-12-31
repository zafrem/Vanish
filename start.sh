#!/bin/bash

# Vanish - Start Script
# Starts the backend and frontend services

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Vanish Start Script ===${NC}"
echo ""

# Check if mode is provided as argument
MODE=${1:-}

if [ -z "$MODE" ]; then
    echo "Usage: ./start.sh [dev|docker|prod]"
    echo ""
    echo "Modes:"
    echo "  dev    - Start in development mode (local Go server + Vite dev server)"
    echo "  docker - Start using Docker Compose (full stack with all services)"
    echo "  prod   - Alias for docker mode"
    echo ""
    exit 1
fi

case "$MODE" in
    dev)
        echo -e "${GREEN}Starting in development mode...${NC}"
        echo ""

        # Check if Redis is needed
        echo -e "${BLUE}Starting Redis (via Docker)...${NC}"
        docker-compose -f docker-compose.dev.yml up -d

        # Wait for Redis to be ready
        echo "Waiting for Redis to be ready..."
        sleep 2

        # Start backend
        echo -e "${BLUE}Starting backend server...${NC}"
        cd backend

        # Check if .env exists
        if [ ! -f .env ]; then
            echo -e "${YELLOW}Warning: .env file not found. Copying from .env.example${NC}"
            if [ -f .env.example ]; then
                cp .env.example .env
                echo "Please edit backend/.env with your configuration"
            fi
        fi

        # Start Go server in background
        nohup go run cmd/server/main.go > ../logs/backend.log 2>&1 &
        BACKEND_PID=$!
        echo $BACKEND_PID > ../logs/backend.pid
        echo -e "${GREEN}Backend started (PID: $BACKEND_PID)${NC}"

        cd ..

        # Wait a bit for backend to start
        sleep 2

        # Start frontend
        echo -e "${BLUE}Starting frontend dev server...${NC}"
        cd frontend

        # Install dependencies if node_modules doesn't exist
        if [ ! -d "node_modules" ]; then
            echo "Installing frontend dependencies..."
            npm install
        fi

        # Start Vite dev server in background
        nohup npm run dev > ../logs/frontend.log 2>&1 &
        FRONTEND_PID=$!
        echo $FRONTEND_PID > ../logs/frontend.pid
        echo -e "${GREEN}Frontend started (PID: $FRONTEND_PID)${NC}"

        cd ..

        echo ""
        echo -e "${GREEN}=== Development servers started! ===${NC}"
        echo -e "Backend:  http://localhost:8080"
        echo -e "Frontend: http://localhost:5173"
        echo ""
        echo "Logs:"
        echo "  Backend:  tail -f logs/backend.log"
        echo "  Frontend: tail -f logs/frontend.log"
        echo ""
        echo "To stop: ./stop.sh dev"
        ;;

    docker|prod)
        echo -e "${GREEN}Starting with Docker Compose...${NC}"
        echo ""

        # Build and start all services
        docker-compose up -d --build

        echo ""
        echo -e "${GREEN}=== Docker services started! ===${NC}"
        echo -e "Frontend: http://localhost:3000"
        echo -e "Backend:  http://localhost:8080"
        echo ""
        echo "View logs:"
        echo "  All services: docker-compose logs -f"
        echo "  Backend only: docker-compose logs -f backend"
        echo "  Frontend only: docker-compose logs -f frontend"
        echo ""
        echo "To stop: ./stop.sh docker"
        ;;

    *)
        echo -e "${YELLOW}Invalid mode: $MODE${NC}"
        echo "Valid modes: dev, docker, prod"
        exit 1
        ;;
esac
