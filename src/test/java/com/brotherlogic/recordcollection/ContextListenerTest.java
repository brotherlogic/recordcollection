package com.brotherlogic.recordcollection;

import java.lang.reflect.Field;
import java.util.Collections;
import java.util.Map;
import java.util.TreeMap;

import javax.servlet.ServletContext;
import javax.servlet.ServletContextEvent;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import org.junit.Assert;
import org.junit.Test;

import org.mockito.Mockito;

public class ContextListenerTest {

  Logger logger = Logger.getLogger(getClass());
  
  @Test
  public void testDestroyed() {
    //Basically this should not fail
    ContextListener listener = new ContextListener();
    ServletContextEvent ev = Mockito.mock(ServletContextEvent.class);
    listener.contextDestroyed(ev);
  }
  
  @Test
  public void testInitialisation() {
    //Setup some dummy values
    Map<String,String> newEnv = new TreeMap<String,String>();
    newEnv.put("discogskey","madeupkey");
    newEnv.put("discogssecret","madeupsecret");
    
    try{
      set(newEnv);
    } catch (Exception e) {
      logger.log(Level.FATAL,"Cannot set environment variables",e);
    }
    
    //This will break up the logging process
    ContextListener listener = new ContextListener();
    ServletContextEvent ev = Mockito.mock(ServletContextEvent.class);
    ServletContext context = Mockito.mock(ServletContext.class);
    Mockito.when(ev.getServletContext()).thenReturn(context);
    listener.contextInitialized(ev);
    
    Mockito.verify(context).setAttribute(Mockito.eq("system"), Mockito.any(System.class));
    Mockito.verify(context).setAttribute(Mockito.eq("token_map"), Mockito.any(Map.class));
  }

  public static void set(Map<String, String> newenv) throws Exception {
    Class[] classes = Collections.class.getDeclaredClasses();
    Map<String, String> env = System.getenv();
    for(Class cl : classes) {
      if("java.util.Collections$UnmodifiableMap".equals(cl.getName())) {
        Field field = cl.getDeclaredField("m");
        field.setAccessible(true);
        Object obj = field.get(env);
        Map<String, String> map = (Map<String, String>) obj;
        map.clear();
        map.putAll(newenv);
      }
    }
  }
}
