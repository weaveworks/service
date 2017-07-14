-- When we change DBcolumn type from text to varchar(100)
-- any fields with existing data beyond new limit are simply truncated
ALTER TABLE organizations ALTER name TYPE varchar(100) USING left(name,100);
