DELIMITER //

CREATE PROCEDURE sp_register_user(
    IN p_name VARCHAR(100),
    IN p_email VARCHAR(150),
    IN p_password_hash VARCHAR(255)
)
BEGIN
    -- Declaramos una variable para manejar errores o estados
    DECLARE user_exists INT DEFAULT 0;

    -- 1. Verificamos si el email ya está registrado
    SELECT COUNT(*) INTO user_exists 
    FROM users 
    WHERE email = p_email;

    IF user_exists > 0 THEN
        -- Si existe, lanzamos un error personalizado
        SIGNAL SQLSTATE '45000' 
        SET MESSAGE_TEXT = 'El correo electrónico ya se encuentra registrado';
    ELSE
        -- 2. Si no existe, insertamos el nuevo usuario
        -- La contraseña p_password_hash ya viene encriptada desde Go
        INSERT INTO users (name, email, password_hash) 
        VALUES (p_name, p_email, p_password_hash);
        
        -- 3. Devolvemos el ID generado
        SELECT LAST_INSERT_ID() AS user_id;
    END IF;
END //

DELIMITER ;