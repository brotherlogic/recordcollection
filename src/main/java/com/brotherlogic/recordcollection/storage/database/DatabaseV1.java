package com.brotherlogic.recordcollection.storage.database;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import java.sql.Connection;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;

public class DatabaseV1 implements Database {

  private Logger logger = Logger.getLogger(DatabaseV1.class);
  
  @Override
  public void create(Connection con) throws SQLException {
    logger.log(Level.INFO,"Create");
    Statement s = con.createStatement();
    s.execute("CREATE TABLE key_table (key char(40) PRIMARY KEY, secret char(40))");
  }

  @Override
  public void destroy(Connection con) throws SQLException {
    logger.log(Level.INFO,"Destroy");
    Statement s = con.createStatement();
    s.execute("DROP TABLE key_table");
  }

  @Override
  public void upgrade(Connection con) throws SQLException {
    logger.log(Level.INFO,"Upgrade");
    create(con);
  }

  @Override
  public boolean validate(Connection con) throws SQLException {
    logger.log(Level.INFO,"Validate");
    Statement s = con.createStatement();
    ResultSet rs = s.executeQuery("SELECT column_name, data_type, character_maximum_length FROM INFORMATION_SCHEMA.COLUMNS where table_name = 'key_table'");

    int seen = 0;
    while(rs.next()) {
      seen++;
      String columnName = rs.getString(1);
      String type = rs.getString(2);
      int len = rs.getInt(3);
      
      logger.log(Level.DEBUG,"Seen V1 " + columnName + " with " + type + " and " + len);
      if (columnName.equals("key") || columnName.equals("secret")) {
        if (!type.equals("character"))
          return false;
        if (len != 40)
          return false;
      } else {
        return false;
      }
    }

    logger.log(Level.DEBUG,"Seen " + seen + " columns in V1");
    
    if (seen > 0)
      return true;
    else
      return false;
  }

  public Database getPrevVersion() {
    return null;
  }
}
