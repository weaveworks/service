ALTER TABLE event_types
  ADD hide_ui_config boolean NOT NULL DEFAULT false;

UPDATE event_types SET hide_ui_config = true, feature_flag = NULL WHERE feature_flag = '__hide_from_ui';
