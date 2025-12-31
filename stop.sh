#!/bin/bash

# Vanish - Stop Script
# Stops the backend and frontend services

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Vanish Stop Script ===${NC}"
echo ""

# Check if mode is provided as argument
MODE=${1:-}

if [ -z "$MODE" ]; then
    echo "Usage: ./stop.sh [dev|docker|prod|all]"
    echo ""
    echo "Modes:"
    echo "  dev    - Stop development mode services"
    echo "  docker - Stop Docker Compose services"
    echo "  prod   - Alias for docker mode"
    echo "  all    - Stop both dev and docker services"
    echo ""
    exit 1
fi

stop_dev() {
    echo -e "${BLUE}Stopping development mode services...${NC}"

    # Stop backend
    if [ -f logs/backend.pid ]; then
        BACKEND_PID=$(cat logs/backend.pid)
        if ps -p $BACKEND_PID > /dev/null 2>&1; then
            echo "Stopping backend (PID: $BACKEND_PID)..."
            kill $BACKEND_PID
            rm logs/backend.pid
            echo -e "${GREEN}Backend stopped${NC}"
        else
            echo -e "${YELLOW}Backend process not found${NC}"
            rm logs/backend.pid
        fi
    else
        echo -e "${YELLOW}Backend PID file not found${NC}"
    fi

    # Stop frontend
    if [ -f logs/frontend.pid ]; then
        FRONTEND_PID=$(cat logs/frontend.pid)
        if ps -p $FRONTEND_PID > /dev/null 2>&1; then
            echo "Stopping frontend (PID: $FRONTEND_PID)..."
            kill $FRONTEND_PID
            rm logs/frontend.pid
            echo -e "${GREEN}Frontend stopped${NC}"
        else
            echo -e "${YELLOW}Frontend process not found${NC}"
            rm logs/frontend.pid
        fi
    else
        echo -e "${YELLOW}Frontend PID file not found${NC}"
    fi

    # Stop Redis dev container
    echo "Stopping Redis development container..."
    docker-compose -f docker-compose.dev.yml down

    echo -e "${GREEN}Development services stopped${NC}"
}

stop_docker() {
    echo -e "${BLUE}Stopping Docker Compose services...${NC}"

    # Stop all Docker services
    docker-compose down

    echo -e "${GREEN}Docker services stopped${NC}"
}

case "$MODE" in
    dev)
        stop_dev
        ;;

    docker|prod)
        stop_docker
        ;;

    all)
        echo -e "${BLUE}Stopping all services...${NC}"
        echo ""
        stop_dev
        echo ""
        stop_docker
        ;;

    *)
        echo -e "${YELLOW}Invalid mode: $MODE${NC}"
        echo "Valid modes: dev, docker, prod, all"
        exit 1
        ;;
esac

echo ""
echo -e "${GREEN}=== All requested services stopped ===${NC}"
