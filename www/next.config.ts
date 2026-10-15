import type { NextConfig } from "next";

// Static export to Cloudflare Pages, served from the apex use.tennis. No
// basePath: the prefix a GitHub project page needs does not exist on a real
// domain, and leaving it set would nest the site under /tennis.
const nextConfig: NextConfig = {
  output: "export",
};

export default nextConfig;
