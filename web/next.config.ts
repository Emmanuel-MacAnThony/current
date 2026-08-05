import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Emit a self-contained server bundle (.next/standalone) so the Docker runtime
  // image is just node + the traced deps, no full node_modules.
  output: "standalone",
};

export default nextConfig;
