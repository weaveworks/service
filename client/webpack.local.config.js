var webpack = require('webpack');
var HtmlWebpackPlugin = require('html-webpack-plugin');

/**
 * This is the Webpack configuration file for local development. It contains
 * local-specific configuration such as the React Hot Loader, as well as:
 *
 * - The entry point of the application
 * - Where the output file should be
 * - Which loaders to use on what files to properly transpile the source
 *
 * For more information, see: http://webpack.github.io/docs/configuration.html
 */

 // Inject websocket url to dev backend
 var WEBPACK_SERVER_HOST = process.env.WEBPACK_SERVER_HOST || 'localhost';

module.exports = {

  // Efficiently evaluate modules with source maps
  devtool: "eval",

  // Set entry point to ./src/main and include necessary files for hot load
  entry:  [
    'webpack-dev-server/client?http://' + WEBPACK_SERVER_HOST + ':4048',
    "webpack/hot/only-dev-server",
    "./src/main"
  ],

  // This will not actually create a bundle.js file in ./build. It is used
  // by the dev server for dynamic hot loading.
  output: {
    path: __dirname + "/build/",
    filename: "app.js",
  },

  // Necessary plugins for hot load
  plugins: [
    new webpack.HotModuleReplacementPlugin(),
    new webpack.NoErrorsPlugin(),
    new HtmlWebpackPlugin({
      template: 'src/html/index.html',
      filename: 'index.html'
    })
  ],

  // Transform source code using Babel and React Hot Loader
  module: {
    preLoaders: [
      {
        test: /\.jsx?$/,
        exclude: /node_modules/,
        loader: 'eslint-loader'
      }
    ],
    loaders: [
      {
        test: /\.woff(2)?(\?v=[0-9]\.[0-9]\.[0-9])?$/,
        loader: 'url-loader?limit=10000&minetype=application/font-woff'
      },
      {
        test: /\.(ttf|eot|svg)(\?v=[0-9]\.[0-9]\.[0-9])?$/,
        loader: 'file-loader'
      },
      {
        test: /\.jsx?$/,
        exclude: /node_modules/,
        loaders: ["react-hot", "babel"],
      }
    ]
  },

  // Automatically transform files with these extensions
  resolve: {
    extensions: ['', '.js', '.jsx']
  }
}
