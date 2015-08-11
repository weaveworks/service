/**
 * Test suite entry point
 */

// Babel Polyfill
import 'babel-core/polyfill';

import './common/test_request';
import './common/test_session';

import './routers/test_logged_out';

import './pages/test_landing_page';