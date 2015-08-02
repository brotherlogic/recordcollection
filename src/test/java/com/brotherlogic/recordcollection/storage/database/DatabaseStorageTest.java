package com.brotherlogic.recordcollection.storage.database;

import com.brotherlogic.recordcollection.BaseTest;

import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;

import org.junit.Assert;
import org.junit.Test;

import org.mockito.Mockito;

import org.scribe.model.Token;

public class DatabaseStorageTest extends BaseTest {

  @Test
  public void testStoreToken() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    PreparedStatement mStatement = Mockito.mock(PreparedStatement.class);
    Mockito.when(mConnection.prepareStatement(Mockito.anyString())).thenReturn(mStatement);

    Token t = new Token("testkey","testsecret");
    DatabaseStorage store = new DatabaseStorage(mConnection);
    store.storeToken(t);

    Mockito.verify(mStatement).execute();
  }

  @Test
  public void testStoreTokenFail() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    PreparedStatement mStatement = Mockito.mock(PreparedStatement.class);
    Mockito.when(mConnection.prepareStatement(Mockito.anyString())).thenReturn(mStatement);
    Mockito.doThrow(new SQLException()).when(mStatement).setString(Mockito.anyInt(),Mockito.anyString());
    Token t = new Token("testkey","testsecret");
    DatabaseStorage store = new DatabaseStorage(mConnection);
    store.storeToken(t);

    Mockito.verify(mStatement, Mockito.never()).execute();
  }

  
  @Test
  public void testGetToken() throws SQLException {
    PreparedStatement mState = Mockito.mock(PreparedStatement.class);
    ResultSet mRes = Mockito.mock(ResultSet.class);
    Mockito.when(mState.executeQuery()).thenReturn(mRes);

    Mockito.when(mRes.next()).thenReturn(true,false);
    Mockito.when(mRes.getString(1)).thenReturn("testsecret");

    Connection mConnection = Mockito.mock(Connection.class);
    DatabaseStorage store = new DatabaseStorage(mConnection);
    Token t = store.getToken("testkey",mState);

    Assert.assertEquals("testsecret",t.getSecret());
  }

  @Test
  public void testGetTokenNullReturn() throws SQLException {
    PreparedStatement mState = Mockito.mock(PreparedStatement.class);
    ResultSet mRes = Mockito.mock(ResultSet.class);
    Mockito.when(mState.executeQuery()).thenReturn(mRes);

    Mockito.when(mRes.next()).thenReturn(false);

    Connection mConnection = Mockito.mock(Connection.class);
    DatabaseStorage store = new DatabaseStorage(mConnection);
    Token t = store.getToken("testkey",mState);

    Assert.assertNull(t);
  }

  @Test
  public void testGetTokenBasic() throws SQLException {
    PreparedStatement mState = Mockito.mock(PreparedStatement.class);
    ResultSet mRes = Mockito.mock(ResultSet.class);
    Mockito.when(mState.executeQuery()).thenReturn(mRes);

    Mockito.when(mRes.next()).thenReturn(true,false);
    Mockito.when(mRes.getString(1)).thenReturn("testsecret");

    Connection mConnection = Mockito.mock(Connection.class);
    Mockito.when(mConnection.prepareStatement(Mockito.anyString())).thenReturn(mState);
    
    DatabaseStorage store = new DatabaseStorage(mConnection);
    Token t = store.getToken("testkey");

    Assert.assertEquals("testsecret",t.getSecret());
  }

  @Test
  public void testGetTokenBasicWithException() throws SQLException {
    PreparedStatement mState = Mockito.mock(PreparedStatement.class);
    ResultSet mRes = Mockito.mock(ResultSet.class);
    Mockito.when(mState.executeQuery()).thenReturn(mRes);
    
    Mockito.when(mRes.next()).thenThrow(new SQLException());
    Mockito.when(mRes.getString(1)).thenReturn("testsecret");
    
    Connection mConnection = Mockito.mock(Connection.class);
    Mockito.when(mConnection.prepareStatement(Mockito.anyString())).thenReturn(mState);
    
    DatabaseStorage store = new DatabaseStorage(mConnection);
    Token t = store.getToken("testkey");

    Assert.assertNull(t);
  }

  
}
