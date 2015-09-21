package com.brotherlogic.recordcollection.storage.database;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import java.sql.Connection;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;

public class DatabaseV2 extends DatabaseV1 implements Database {

  private Logger logger = Logger.getLogger(DatabaseV2.class);

  @Override
  public void create(Connection con) throws SQLException {
    logger.log(Level.INFO,"Create");
    super.create(con);
    create(con,true);
  }
  
  protected void create(Connection con, boolean past) throws SQLException {
    logger.log(Level.INFO,"CREATE " + past);
    Statement s = con.createStatement();
    s.execute("CREATE TABLE collection (userid integer, name char(60), folders char(60), proportions char(60))");
  }

  @Override
  public void destroy(Connection con) throws SQLException {
    logger.log(Level.INFO,"Destroy");
    super.destroy(con);
    Statement s = con.createStatement();
    s.execute("DROP TABLE collection");
  }

  @Override
  public void upgrade(Connection con) throws SQLException {
    logger.log(Level.INFO,"Upgrade");
    create(con,false);
  }

  @Override
  public boolean validate(Connection con) throws SQLException {

    logger.log(Level.INFO,"Validate");
    
    if (!super.validate(con)) {
      logger.log(Level.INFO,"Super failed validation");
      return false;
    }

    logger.log(Level.INFO,"Running test query");
    Statement s = con.createStatement();
    ResultSet rs = s.executeQuery("SELECT column_name, data_type, character_maximum_length FROM INFORMATION_SCHEMA.COLUMNS where table_name = 'collection'");
    logger.log(Level.INFO,"Run test query");
    
    int seen = 0;
    logger.log(Level.INFO,"Starting analysis " + rs);
    while(rs.next()) {
      logger.log(Level.INFO,"FOund " + rs);
      seen++;
      String columnName = rs.getString(1);
      String type = rs.getString(2);
      int len = rs.getInt(3);

      logger.log(Level.DEBUG,"V2 Seen " + columnName + " with " + type + " and " + len);
      if (columnName.equals("name") || columnName.equals("folders") || columnName.equals("proportions")) {
        if (!type.equals("character")) {
          logger.log(Level.DEBUG,"Bad type");
          return false;
        }
        if (len != 60) {
          logger.log(Level.DEBUG,"Bad number");
          return false;
        }
      } else if (columnName.equals("userid")) {
        if (!type.equals("integer")) {
          logger.log(Level.DEBUG,"Bad Integer");
          return false;
        }
      }
      else {
        logger.log(Level.DEBUG,"Bad name");
        return false;
      }
    }

    logger.log(Level.INFO,"Seen " + seen + " columns in V2");
    
    if (seen > 0)
      return true;
    else
      return false;
  }

  public Database getPrevVersion() {
    return new DatabaseV1();
  }
}
