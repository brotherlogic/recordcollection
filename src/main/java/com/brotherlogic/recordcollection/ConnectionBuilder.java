package com.brotherlogic.recordcollection;

import java.sql.Connection;
import java.sql.SQLException;

public interface ConnectionBuilder {
  Connection makeConnection(String url, String username, String password) throws SQLException;
}
