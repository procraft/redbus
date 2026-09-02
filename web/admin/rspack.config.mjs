import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { rspack } from '@rspack/core';
import { ReactRefreshRspackPlugin } from '@rspack/plugin-react-refresh';
import dotenv from 'dotenv';

const currentDirectory = path.dirname(fileURLToPath(import.meta.url));
const isDevelopment = process.env.NODE_ENV === 'development';

dotenv.config({ path: path.join(currentDirectory, '.env'), quiet: true });

export default {
  context: currentDirectory,
  mode: isDevelopment ? 'development' : 'production',
  performance: {
    hints: 'warning',
    maxAssetSize: 650 * 1024,
    maxEntrypointSize: 1100 * 1024,
  },
  entry: './src/main.tsx',
  devtool: isDevelopment ? 'eval-cheap-module-source-map' : false,
  output: {
    path: path.join(currentDirectory, 'dist'),
    clean: true,
    filename: isDevelopment ? '[name].js' : 'assets/[name].[contenthash:8].js',
    cssFilename: isDevelopment ? '[name].css' : 'assets/[name].[contenthash:8].css',
  },
  resolve: {
    extensions: ['.tsx', '.ts', '.jsx', '.js'],
    alias: {
      '@': path.join(currentDirectory, 'src'),
    },
  },
  module: {
    rules: [
      {
        test: /\.(?:js|jsx|ts|tsx)$/,
        exclude: /node_modules/,
        use: {
          loader: 'builtin:swc-loader',
          options: {
            jsc: {
              parser: {
                syntax: 'typescript',
                tsx: true,
              },
              transform: {
                react: {
                  development: isDevelopment,
                  refresh: isDevelopment,
                  runtime: 'automatic',
                },
              },
            },
          },
        },
        type: 'javascript/auto',
      },
      {
        test: /\.css$/i,
        type: 'css/auto',
      },
    ],
  },
  plugins: [
    new rspack.HtmlRspackPlugin({
      template: './public/index.html',
    }),
    new rspack.CopyRspackPlugin({
      patterns: [
        { from: './public/favicon.ico', to: 'favicon.ico' },
        { from: './public/logo.png', to: 'logo.png' },
        { from: './public/runtime-config.js', to: 'runtime-config.js' },
      ],
    }),
    new rspack.DefinePlugin({
      __REDBUS_API_HOST__: JSON.stringify(process.env.REDBUS_API_HOST ?? ''),
      __REDBUS_API_TOKEN__: JSON.stringify(process.env.REDBUS_API_TOKEN ?? ''),
    }),
    isDevelopment && new ReactRefreshRspackPlugin(),
  ].filter(Boolean),
  devServer: {
    port: 8081,
    hot: true,
    historyApiFallback: true,
  },
};
