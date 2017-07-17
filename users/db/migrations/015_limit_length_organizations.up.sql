-- When we change DBcolumn type from text to varchar(100)
-- any fields with existing data beyond new limit are simply truncated

-- Constraint here needs to match UI-enforced length constraint see
-- service-ui:client/src/common/constants.js:INSTANCE_NAME_MAX_LENGTH
ALTER TABLE organizations ALTER name TYPE varchar(100) USING left(name,100);
