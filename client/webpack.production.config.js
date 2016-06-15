var webpack = require('webpack');
var CleanWebpackPlugin = require('clean-webpack-plugin');
var HtmlWebpackPlugin = require('html-webpack-plugin');

/**
 * This is the Webpack configuration file for production.
 */
module.exports = {
  entry: {
    app: './src/main',
    // keep only some in here, to make vendors and app bundles roughly same size
    vendors: ['babel-polyfill', 'debug', 'history', 'lodash', 'moment',
      'react', 'react-dom', 'react-motion', 'react-router']
  },

  output: {
    path: __dirname + '/build/',
    filename: '[name].js'
  },

  module: {
    loaders: [
      {
        test: /\.woff(2)?(\?v=[0-9]\.[0-9]\.[0-9])?$/,
        loader: 'url-loader?limit=10000&minetype=application/font-woff'
      },
      {
        test: /\.(ttf|eot|svg)(\?v=[0-9]\.[0-9]\.[0-9])?$/,
        loader: 'file-loader'
      },
      { test: /\.jsx?$/, exclude: /node_modules/, loader: 'babel' }
    ]
  },

  resolve: {
    extensions: ['', '.js', '.jsx']
  },

  plugins: [
    new CleanWebpackPlugin(['build']),
    new webpack.DefinePlugin({
      'process.env': {NODE_ENV: '"production"'}
    }),
    new webpack.optimize.CommonsChunkPlugin('vendors', 'vendors.js'),
    new webpack.optimize.OccurenceOrderPlugin(true),
    new webpack.IgnorePlugin(/^\.\/locale$/, [/moment$/]),
    new webpack.IgnorePlugin(/^\.\/svg-icons$/, [/material-ui$/]),
    new webpack.optimize.UglifyJsPlugin({
      sourceMap: false,
      compress: {
        warnings: false
      }
    }),
    new HtmlWebpackPlugin({
      hash: true,
      chunks: ['vendors', 'app'],
      template: 'src/html/index.html',
      filename: 'index.html'
    })
  ]
};
