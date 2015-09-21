package com.brotherlogic.recordcollection.storage.database;

import com.brotherlogic.recordcollection.ConnectionBuilder;

import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.SQLException;

public class DatabaseConnector implements ConnectionBuilder {
  public Connection makeConnection(String url, String user, String pass) throws SQLException {
    return DriverManager.getConnection(url,user,pass);
  }
}
