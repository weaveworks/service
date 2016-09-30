var webpack = require('webpack');
var CleanWebpackPlugin = require('clean-webpack-plugin');
var HtmlWebpackPlugin = require('html-webpack-plugin');

/**
 * This is the Webpack configuration file for building a static image to be used in a local env.
 * It is identical to the production config, except paths are relative to the current host,
 * instead of pointing to S3. It outputs to ./build-local
 */
module.exports = {
  entry: {
    app: './src/main',
    // keep only some in here, to make vendors and app bundles roughly same size
    vendors: ['babel-polyfill', 'debug', 'react', 'react-dom', 'react-router',
      'redux', 'react-redux', 'redux-thunk']
  },

  output: {
    // absolute path for bundle
    publicPath: '/',
    path: __dirname + '/build-local/',
    filename: '[chunkhash].js'
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
    new webpack.optimize.CommonsChunkPlugin('vendors', '[chunkhash].js'),
    new webpack.optimize.OccurenceOrderPlugin(true),
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
