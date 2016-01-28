package com.brotherlogic.recordcollection.storage.database;

import java.util.Arrays;
import java.util.List;
import java.util.LinkedList;

import com.brotherlogic.recordcollection.BaseTest;
import com.brotherlogic.recordcollection.FolderGroup;

import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;

import org.junit.Assert;
import org.junit.Test;

import org.mockito.Mockito;

import org.scribe.model.Token;

public class DatabaseStorageTest extends BaseTest {

  @Test
  public void testStoreCollection() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    PreparedStatement mStatement = Mockito.mock(PreparedStatement.class);
    Mockito.when(mConnection.prepareStatement(Mockito.anyString())).thenReturn(mStatement);

    FolderGroup c = new FolderGroup(Arrays.asList(new Integer[] {12,13,14}), Arrays.asList(new Integer[] {12,13,15}), "TestCollection");
    DatabaseStorage store = new DatabaseStorage(mConnection);
    store.storeCollection(123,c);

    Mockito.verify(mStatement).execute();
  }

  @Test
  public void testStoreCollectionWithException() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    PreparedStatement mStatement = Mockito.mock(PreparedStatement.class);
    Mockito.when(mConnection.prepareStatement(Mockito.anyString())).thenReturn(mStatement);
    Mockito.doThrow(new SQLException()).when(mStatement).setString(Mockito.anyInt(),Mockito.anyString());
    
    FolderGroup c = new FolderGroup(Arrays.asList(new Integer[] {12,13,14}), Arrays.asList(new Integer[] {12,13,15}), "TestCollection");
    DatabaseStorage store = new DatabaseStorage(mConnection);
    store.storeCollection(123,c);

    Mockito.verify(mStatement, Mockito.never()).execute();
  }
  
  @Test
  public void testListConvert() throws SQLException {
    List<Integer> vals = new LinkedList<Integer>();
    vals.add(15);
    vals.add(12);

    DatabaseStorage storage = new DatabaseStorage(Mockito.mock(Connection.class));
    String res = storage.convertList(vals);
    Assert.assertEquals("12,15",res);
  }

  @Test
  public void testListConvertEmpty() throws SQLException {
    DatabaseStorage storage = new DatabaseStorage(Mockito.mock(Connection.class));
    String res = storage.convertList(new LinkedList<Integer>());
    Assert.assertEquals("",res);
  }

  @Test
  public void testConvertString() throws SQLException {
    DatabaseStorage storage = new DatabaseStorage(Mockito.mock(Connection.class));
    List<Integer> vals = storage.convertString("15,12");
    Assert.assertEquals(2,vals.size());
    Assert.assertEquals(new Integer(15),vals.get(0));
    Assert.assertEquals(new Integer(12),vals.get(1));
  }
  
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
  public void testGetCollection() throws SQLException {
    PreparedStatement mState = Mockito.mock(PreparedStatement.class);
    ResultSet mRes = Mockito.mock(ResultSet.class);
    Mockito.when(mState.executeQuery()).thenReturn(mRes);

    Mockito.when(mRes.next()).thenReturn(true,false);
    Mockito.when(mRes.getString(1)).thenReturn("12,14");
    Mockito.when(mRes.getString(2)).thenReturn("15,16");

    Connection mConnection = Mockito.mock(Connection.class);
    DatabaseStorage store = new DatabaseStorage(mConnection);
    FolderGroup recCol = store.getCollection(1234,"Test",mState);

    List<Integer> vals = Arrays.asList(new Integer[] {12,14});
    List<Integer> vals2 = Arrays.asList(new Integer[] {15,16});

    Assert.assertEquals(vals,recCol.getFolders());
    Assert.assertEquals(vals2,recCol.getProps());
  }

  @Test
  public void testGetCollections() throws SQLException {
    PreparedStatement mState = Mockito.mock(PreparedStatement.class);
    ResultSet mRes = Mockito.mock(ResultSet.class);
    Mockito.when(mState.executeQuery()).thenReturn(mRes);
    
    Mockito.when(mRes.next()).thenReturn(true,true,false);
    Mockito.when(mRes.getString(1)).thenReturn("12,15","14,16");
    Mockito.when(mRes.getString(2)).thenReturn("13,16","12,13");
    Mockito.when(mRes.getString(3)).thenReturn("Blah1","Blah2");

    Connection mConnection = Mockito.mock(Connection.class);
    Mockito.when(mConnection.prepareStatement(Mockito.anyString())).thenReturn(mState);
    DatabaseStorage store = new DatabaseStorage(mConnection);
    List<FolderGroup> cols = store.getCollections(1234);

    Assert.assertEquals(2,cols.size());
  }

  @Test
  public void testGetCollectionsWithException() throws SQLException {
    PreparedStatement mState = Mockito.mock(PreparedStatement.class);
    ResultSet mRes = Mockito.mock(ResultSet.class);
    Mockito.when(mState.executeQuery()).thenReturn(mRes);
    
    Mockito.when(mRes.next()).thenThrow(new SQLException());
    Mockito.when(mRes.getString(1)).thenReturn("12,15","14,16");
    Mockito.when(mRes.getString(2)).thenReturn("13,16","12,13");
    Mockito.when(mRes.getString(3)).thenReturn("Blah1","Blah2");

    Connection mConnection = Mockito.mock(Connection.class);
    Mockito.when(mConnection.prepareStatement(Mockito.anyString())).thenReturn(mState);
    DatabaseStorage store = new DatabaseStorage(mConnection);
    List<FolderGroup> cols = store.getCollections(1234);

    Assert.assertNull(cols);
  }
  
  @Test
  public void testGetCollectionWrongName() throws SQLException{
    PreparedStatement mState = Mockito.mock(PreparedStatement.class);
    ResultSet mRes = Mockito.mock(ResultSet.class);
    Mockito.when(mState.executeQuery()).thenReturn(mRes);

    Mockito.when(mRes.next()).thenReturn(false);
    Mockito.when(mRes.getString(1)).thenReturn("12,14");
    Mockito.when(mRes.getString(2)).thenReturn("15,16");

    Connection mConnection = Mockito.mock(Connection.class);
    DatabaseStorage store = new DatabaseStorage(mConnection);
    FolderGroup recCol = store.getCollection(1234,"TestMadeUp",mState);

    Assert.assertNull(recCol);
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
  public void testGetCollectionBasic() throws SQLException {
    PreparedStatement mState = Mockito.mock(PreparedStatement.class);
    ResultSet mRes = Mockito.mock(ResultSet.class);
    Mockito.when(mState.executeQuery()).thenReturn(mRes);

    Mockito.when(mRes.next()).thenReturn(true,true,false);
    Mockito.when(mRes.getString(1)).thenReturn("12,14");
    Mockito.when(mRes.getString(2)).thenReturn("13,14");

    Connection mConnection = Mockito.mock(Connection.class);
    Mockito.when(mConnection.prepareStatement(Mockito.anyString())).thenReturn(mState);
    
    DatabaseStorage store = new DatabaseStorage(mConnection);
    FolderGroup rc = store.getCollection(1234,"testkey");

    Assert.assertEquals("testkey",rc.getName());
  }

  @Test
  public void testForceClean() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);

    Statement mState = Mockito.mock(Statement.class);
    ResultSet mRes = Mockito.mock(ResultSet.class);
    Mockito.when(mState.executeQuery(Mockito.anyString())).thenReturn(mRes);
    Mockito.when(mRes.next()).thenReturn(true,false);
    Mockito.when(mRes.getString(1)).thenReturn("madeuptable");

    Statement mState2 = Mockito.mock(Statement.class);
    Mockito.when(mConnection.createStatement()).thenReturn(mState,mState2);

    DatabaseStorage store = new DatabaseStorage(mConnection);
    store.forceCleanDatabase();
    Mockito.verify(mState2).execute("DROP TABLE madeuptable");
  }

  @Test
  public void testForceCleanBadCall() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);

    Statement mState = Mockito.mock(Statement.class);
    ResultSet mRes = Mockito.mock(ResultSet.class);
    Mockito.when(mRes.next()).thenThrow(new SQLException());
    Mockito.when(mState.executeQuery(Mockito.anyString())).thenReturn(mRes);
    Statement mState2 = Mockito.mock(Statement.class);
    Mockito.when(mConnection.createStatement()).thenReturn(mState,mState2);

    DatabaseStorage store = new DatabaseStorage(mConnection);
    store.forceCleanDatabase();
    Mockito.verify(mState2, Mockito.never()).execute(Mockito.anyString());
  }

  
  @Test
  public void testGetCollectionBasicWithException() throws SQLException {
    PreparedStatement mState = Mockito.mock(PreparedStatement.class);
    ResultSet mRes = Mockito.mock(ResultSet.class);
    Mockito.when(mState.executeQuery()).thenReturn(mRes);

    Mockito.when(mRes.next()).thenThrow(new SQLException());
    Mockito.when(mRes.getString(1)).thenReturn("12,14");
    Mockito.when(mRes.getString(2)).thenReturn("13,14");

    Connection mConnection = Mockito.mock(Connection.class);
    Mockito.when(mConnection.prepareStatement(Mockito.anyString())).thenReturn(mState);
    
    DatabaseStorage store = new DatabaseStorage(mConnection);
    FolderGroup rc = store.getCollection(1234,"testkey");

    Assert.assertNull(rc);
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
