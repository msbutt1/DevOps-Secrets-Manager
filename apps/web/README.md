# DevOps Secrets Manager - Web Interface

React-based web interface for the DevOps Secrets Manager platform.

## Tech Stack

- React 18
- TypeScript
- Vite
- TailwindCSS
- shadcn/ui components
- React Query for data fetching
- React Router for navigation

## Development

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build

# Run tests
npm test
```

## Environment Variables

Create a `.env` file:

```
VITE_API_BASE_URL=/api
```

For production, set this to the full API URL.

## Project Structure

```
src/
  components/     # Reusable UI components
  pages/          # Route pages
  hooks/          # Custom React hooks
  lib/            # Utilities and API client
  contexts/       # React contexts
  types/          # TypeScript types
```
