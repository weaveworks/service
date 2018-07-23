ALTER TABLE event_types
  ADD hidden_receiver_types text[] NOT NULL DEFAULT '{}';
