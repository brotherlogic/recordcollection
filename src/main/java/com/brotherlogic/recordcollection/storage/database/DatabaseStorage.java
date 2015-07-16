package com.brotherlogic.recordcollection.storage.database;

import com.brotherlogic.recordcollection.storage.Storage;
import com.brotherlogic.recordcollection.RequestBuilder;
import com.brotherlogic.recordcollection.ScribeRetriever;

import java.util.Map;
import java.util.TreeMap;

import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.Statement;
import java.sql.SQLException;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import org.scribe.oauth.OAuthService;
import org.scribe.model.Token;

public class DatabaseStorage implements Storage {

  private Connection connection;
  private PreparedStatement getTokenStmt;
  private PreparedStatement storeTokenStmt;
  private Logger logger = Logger.getLogger(getClass());
  
  public DatabaseStorage(Connection conn) throws SQLException {
    connection = conn;
    getTokenStmt = connection.prepareStatement("SELECT secret FROM key_table where key = ?");
    storeTokenStmt = connection.prepareStatement("INSERT INTO key_table (key,secret) VALUES (?,?)");
  }

  @Override
  public void storeToken(Token tok) {
    try {
      storeTokenStmt.setString(1,tok.getToken());
      storeTokenStmt.setString(2,tok.getSecret());
      storeTokenStmt.execute();
    } catch (SQLException e) {
      logger.log(Level.ERROR,"Cannot store token",e);
    }
  }
  
  @Override
  public Token getToken(String userKey) {
    try {
      return getToken(userKey, getTokenStmt);
    } catch (SQLException e) {
      logger.log(Level.ERROR,"Failed to run get token",e);
    }

    return null;
  }
  
  protected Token getToken(String userKey, PreparedStatement s) throws SQLException {
    s.setString(1,userKey);
    ResultSet rs = s.executeQuery();

    if (rs.next()) {
      String secret = rs.getString(1).trim();      
      Token tok = new Token(userKey,secret);
      return tok;
    }

    return null;
  }
}
